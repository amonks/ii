package todo

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/amonks/incrementum/internal/db"
	internalids "github.com/amonks/incrementum/internal/ids"
	"github.com/amonks/incrementum/internal/jj"
	"github.com/amonks/incrementum/internal/paths"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/workspace"
	"golang.org/x/term"
)

const (
	// BookmarkName is the jj bookmark used to identify the todo store change.
	BookmarkName = "incr/tasks"

	// TodosFile is the name of the JSONL file containing todos.
	TodosFile = "todos.jsonl"

	// DependenciesFile is the name of the JSONL file containing dependencies.
	DependenciesFile = "dependencies.jsonl"

	maxJSONLineBytes = 1024 * 1024
	jsonlBufferSize  = 64 * 1024
)

var jsonlReaderPool = sync.Pool{
	New: func() any {
		return bufio.NewReaderSize(bytes.NewReader(nil), jsonlBufferSize)
	},
}

var jsonlWriterPool = sync.Pool{
	New: func() any {
		return bufio.NewWriterSize(io.Discard, jsonlBufferSize)
	},
}

var jsonlLineBufPool = sync.Pool{
	New: func() any {
		return make([]byte, 0, jsonlBufferSize)
	},
}

var jsonlTodoBufPool = sync.Pool{
	New: func() any {
		return make([]byte, 0, 512)
	},
}

var jsonlDependencyBufPool = sync.Pool{
	New: func() any {
		return make([]byte, 0, 128)
	},
}

// Store provides access to the todo data for a jujutsu repository.
// It manages workspace acquisition and file locking for concurrent access.
type Store struct {
	repoPath  string
	wsPath    string
	pool      *workspace.Pool
	snapshot  Snapshotter
	prompter  Prompter
	client    *jj.Client
	readOnly  bool
	wsRelease func() error
	lockFile  *os.File
}

// Snapshotter records workspace changes.
type Snapshotter interface {
	Snapshot(workspacePath string) error
}

// Prompter is used to ask the user for confirmation.
type Prompter interface {
	// Confirm asks the user a yes/no question and returns true if they say yes.
	Confirm(message string) (bool, error)
}

// StdioPrompter implements Prompter using stdin for input and stderr for output.
type StdioPrompter struct{}

// Confirm asks the user a yes/no question. Writes prompts to stderr for
// consistency with error messages in interactive error-recovery scenarios;
// reads responses from stdin via fmt.Scanln.
func (p StdioPrompter) Confirm(message string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s [y/n]: ", message)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false, err
	}
	return response == "y" || response == "Y" || response == "yes" || response == "Yes", nil
}

// OpenOptions configures how the store is opened.
type OpenOptions struct {
	// Prompter is used for user confirmation. If nil, StdioPrompter is used.
	Prompter Prompter

	// CreateIfMissing creates the todo store if it doesn't exist.
	// If false and the store doesn't exist, ErrNoTodoStore is returned.
	CreateIfMissing bool

	// PromptToCreate prompts the user before creating a new store.
	// Only used when CreateIfMissing is true.
	PromptToCreate bool

	// Purpose describes why the store workspace is acquired.
	// If empty, a default purpose is used.
	Purpose string

	// ReadOnly opens the store without acquiring a workspace.
	// Read-only mode cannot create missing stores.
	ReadOnly bool
}

// Open opens the todo store for the repository at repoPath.
// If the incr/tasks bookmark doesn't exist and PromptToCreate is true,
// the user will be prompted to create it.
func Open(repoPath string, opts OpenOptions) (*Store, error) {
	usesStdioPrompter := opts.Prompter == nil
	if opts.Prompter == nil {
		opts.Prompter = StdioPrompter{}
	}

	purpose := internalstrings.NormalizeWhitespace(opts.Purpose)
	if purpose == "" {
		purpose = "todo store"
	}

	client := jj.New()

	// Update stale working copy before listing bookmarks
	// This can happen if another workspace has modified the repo
	_ = client.WorkspaceUpdateStale(repoPath)

	// Check if the bookmark exists
	bookmarks, err := client.BookmarkList(repoPath)
	if err != nil {
		return nil, fmt.Errorf("list bookmarks: %w", err)
	}

	hasBookmark := slices.Contains(bookmarks, BookmarkName)

	if !hasBookmark {
		if opts.ReadOnly || !opts.CreateIfMissing {
			return nil, ErrNoTodoStore
		}

		if opts.PromptToCreate {
			shouldPrompt := !usesStdioPrompter || term.IsTerminal(int(os.Stdin.Fd()))
			if shouldPrompt {
				confirmed, err := opts.Prompter.Confirm("No todo store found. Create one?")
				if err != nil {
					return nil, fmt.Errorf("prompt: %w", err)
				}
				if !confirmed {
					return nil, ErrNoTodoStore
				}
			}
		}
	}

	if opts.ReadOnly {
		return &Store{
			repoPath: repoPath,
			client:   client,
			prompter: opts.Prompter,
			readOnly: true,
		}, nil
	}

	lockFile, err := acquireTodoLock(repoPath)
	if err != nil {
		return nil, err
	}

	pool, err := workspace.Open()
	if err != nil {
		releaseTodoLock(lockFile)
		return nil, fmt.Errorf("open workspace pool: %w", err)
	}

	// Acquire a workspace. If the bookmark doesn't exist yet, we'll create
	// the store in this workspace, then edit to it.
	wsPath, err := pool.Acquire(repoPath, workspace.AcquireOptions{Purpose: purpose, SkipHooks: true})
	if err != nil {
		releaseTodoLock(lockFile)
		return nil, fmt.Errorf("acquire workspace: %w", err)
	}

	// If the bookmark doesn't exist, create the store in our workspace
	if !hasBookmark {
		if err := createTodoStore(client, wsPath); err != nil {
			pool.Release(wsPath)
			releaseTodoLock(lockFile)
			return nil, fmt.Errorf("create todo store: %w", err)
		}
	}

	// Edit to the bookmark
	if err := client.Edit(wsPath, BookmarkName); err != nil {
		pool.Release(wsPath)
		releaseTodoLock(lockFile)
		return nil, fmt.Errorf("edit to todo store: %w", err)
	}

	// Update stale working copy if needed (this can happen if the bookmark
	// was moved after the workspace was last used)
	if err := client.WorkspaceUpdateStale(wsPath); err != nil {
		// Ignore errors - the workspace might not be stale
		// The error message is not helpful for detecting this case
	}

	return &Store{
		repoPath: repoPath,
		wsPath:   wsPath,
		pool:     pool,
		snapshot: client,
		prompter: opts.Prompter,
		client:   client,
		wsRelease: func() error {
			return pool.Release(wsPath)
		},
		lockFile: lockFile,
	}, nil
}

// Release releases the workspace back to the pool.
// This should be called when done using the store.
func (s *Store) Release() error {
	if s.wsRelease != nil {
		releaseErr := s.wsRelease()
		lockErr := releaseTodoLock(s.lockFile)
		return errors.Join(releaseErr, lockErr)
	}
	return releaseTodoLock(s.lockFile)
}

// createTodoStore creates the orphan change and bookmark for the todo store.
// wsPath must be an already-acquired workspace.
func createTodoStore(client *jj.Client, wsPath string) error {
	// Create a new change at root() in the workspace.
	// This moves the workspace's @ to the new orphan change.
	changeID, err := client.NewChange(wsPath, "root()")
	if err != nil {
		return fmt.Errorf("create orphan change: %w", err)
	}

	// Set the description
	if err := client.Describe(wsPath, "ii todo store - do not edit directly"); err != nil {
		return fmt.Errorf("describe change: %w", err)
	}

	// Create the bookmark pointing to the new change.
	// This bookmark is visible repo-wide, not just in this workspace.
	if err := client.BookmarkCreate(wsPath, BookmarkName, changeID); err != nil {
		return fmt.Errorf("create bookmark: %w", err)
	}

	return nil
}

func storeFilePath(wsPath, filename string) string {
	return filepath.Join(wsPath, filename)
}

// withFileLock executes fn while holding an exclusive lock on the file at path.
// Creates the file if it doesn't exist.
func withFileLock(path string, fn func(*os.File) error) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	// Open or create the file
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open file for locking: %w", err)
	}
	defer f.Close()

	// Acquire exclusive lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn(f)
}

func readJSONLFromReader[T any](reader io.Reader) ([]T, error) {
	var items []T
	readerSize := estimateReaderSize(reader)
	if err := scanJSONLReader(reader, func(line []byte, itemIndex int) (bool, error) {
		if items == nil && readerSize > 0 {
			items = make([]T, 0, estimateJSONLItems(readerSize, len(line)))
		}
		item, err := decodeJSONLItem[T](line, itemIndex)
		if err != nil {
			return false, err
		}
		items = append(items, item)
		return false, nil
	}); err != nil {
		return nil, err
	}

	return items, nil
}

func readTodosByExactIDsFromReader(reader io.Reader, missing map[string]struct{}) (map[string]Todo, error) {
	items := make(map[string]Todo, len(missing))
	if err := scanJSONLReader(reader, func(line []byte, itemIndex int) (bool, error) {
		item, err := decodeJSONLItem[Todo](line, itemIndex)
		if err != nil {
			return false, err
		}
		if _, ok := missing[item.ID]; ok {
			items[item.ID] = item
			delete(missing, item.ID)
			if len(missing) == 0 {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return nil, err
	}

	return items, nil
}

func decodeJSONLItem[T any](line []byte, itemIndex int) (T, error) {
	var item T
	if err := json.Unmarshal(line, &item); err != nil {
		return item, fmt.Errorf("decode item %d: %w", itemIndex+1, err)
	}
	return item, nil
}

type readerStat interface {
	Stat() (os.FileInfo, error)
}

type readerSizer interface {
	Size() int64
}

type readerLener interface {
	Len() int
}

func estimateReaderSize(reader io.Reader) int64 {
	switch r := reader.(type) {
	case readerSizer:
		return r.Size()
	case readerLener:
		return int64(r.Len())
	case readerStat:
		info, err := r.Stat()
		if err != nil {
			return 0
		}
		return info.Size()
	default:
		return 0
	}
}

func estimateJSONLItems(totalSize int64, lineSize int) int {
	if totalSize <= 0 || lineSize <= 0 {
		return 0
	}
	estimated := totalSize / int64(lineSize)
	maxInt := int(^uint(0) >> 1)
	if estimated > int64(maxInt) {
		estimated = int64(maxInt)
	}
	if estimated < 1 {
		estimated = 1
	}
	return int(estimated)
}

func scanJSONLReader(reader io.Reader, handle func(line []byte, itemIndex int) (bool, error)) error {
	buf := jsonlReaderPool.Get().(*bufio.Reader)
	buf.Reset(reader)
	defer func() {
		buf.Reset(bytes.NewReader(nil))
		jsonlReaderPool.Put(buf)
	}()
	lineBuf := jsonlLineBufPool.Get().([]byte)
	lineBuf = lineBuf[:0]
	defer func() {
		if cap(lineBuf) <= jsonlBufferSize {
			jsonlLineBufPool.Put(lineBuf[:0]) //lint:ignore SA6002 []byte pool is intentional; pointer indirection adds complexity without meaningful benefit here
		}
	}()
	itemIndex := 0
	for {
		line, nextBuf, err := readJSONLLine(buf, lineBuf)
		lineBuf = nextBuf
		atEOF := errors.Is(err, io.EOF)
		if err != nil && !atEOF {
			return fmt.Errorf("decode item %d: %w", itemIndex+1, err)
		}
		if len(line) == 0 {
			if atEOF {
				break
			}
			continue
		}
		done, err := handle(line, itemIndex)
		if err != nil {
			return err
		}
		itemIndex++
		if done || atEOF {
			break
		}
	}

	return nil
}

func readJSONLLine(reader *bufio.Reader, lineBuf []byte) ([]byte, []byte, error) {
	lineBuf = lineBuf[:0]
	for {
		chunk, isPrefix, err := reader.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(chunk) == 0 && len(lineBuf) == 0 {
					return nil, lineBuf, io.EOF
				}
			} else {
				return nil, lineBuf, err
			}
		}
		if len(lineBuf) == 0 && !isPrefix {
			if len(chunk) > maxJSONLineBytes {
				return nil, lineBuf, fmt.Errorf("exceeds max JSON line size")
			}
			return chunk, lineBuf, err
		}
		lineBuf = append(lineBuf, chunk...)
		if len(lineBuf) > maxJSONLineBytes {
			return nil, lineBuf, fmt.Errorf("exceeds max JSON line size")
		}
		if !isPrefix {
			return lineBuf, lineBuf, err
		}
	}
}

// writeJSONL writes a slice of items to a JSONL file, overwriting any existing content.
func writeJSONL[T any](path string, items []T) error {
	switch typed := any(items).(type) {
	case []Todo:
		return writeJSONLWithWriter(path, func(writer *bufio.Writer) error {
			return writeJSONLWithBufferPool(writer, typed, &jsonlTodoBufPool, appendTodoJSONLine)
		})
	case []Dependency:
		return writeJSONLWithWriter(path, func(writer *bufio.Writer) error {
			return writeJSONLWithBufferPool(writer, typed, &jsonlDependencyBufPool, appendDependencyJSONLine)
		})
	default:
		return writeJSONLWithWriter(path, func(writer *bufio.Writer) error {
			encoder := json.NewEncoder(writer)
			encoder.SetEscapeHTML(false)
			for i, item := range items {
				if err := encoder.Encode(item); err != nil {
					return fmt.Errorf("encode item %d: %w", i, err)
				}
			}
			return nil
		})
	}
}

func writeJSONLWithBufferPool[T any](writer *bufio.Writer, items []T, pool *sync.Pool, appendLine func([]byte, *T) []byte) error {
	buf := pool.Get().([]byte)
	defer func() {
		if cap(buf) <= jsonlBufferSize {
			pool.Put(buf[:0]) //lint:ignore SA6002 []byte pool is intentional
		}
	}()
	for i := range items {
		buf = buf[:0]
		buf = appendLine(buf, &items[i])
		if _, err := writer.Write(buf); err != nil {
			return fmt.Errorf("encode item %d: %w", i, err)
		}
	}
	return nil
}

func writeJSONLWithWriter(path string, write func(*bufio.Writer) error) error {
	// Write to temp file first
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	buffered := jsonlWriterPool.Get().(*bufio.Writer)
	buffered.Reset(f)
	defer func() {
		buffered.Reset(io.Discard)
		jsonlWriterPool.Put(buffered)
	}()

	if err := write(buffered); err != nil {
		closeErr := closeAndRemoveTempFile(tmpPath, f)
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if err := buffered.Flush(); err != nil {
		closeErr := closeAndRemoveTempFile(tmpPath, f)
		if closeErr != nil {
			return errors.Join(fmt.Errorf("flush temp file: %w", err), closeErr)
		}
		return fmt.Errorf("flush temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		removeTempFile(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		removeTempFile(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func removeTempFile(path string) {
	_ = os.Remove(path)
}

func closeAndRemoveTempFile(path string, file *os.File) error {
	closeErr := file.Close()
	removeTempFile(path)
	return closeErr
}

var jsonHexDigits = []byte("0123456789abcdef")

func appendTodoJSONLine(buf []byte, todo *Todo) []byte {
	buf = append(buf, '{')
	hasField := false

	buf, hasField = appendJSONFieldPrefix(buf, "id", hasField)
	buf = appendJSONString(buf, todo.ID)

	buf, hasField = appendJSONFieldPrefix(buf, "title", hasField)
	buf = appendJSONString(buf, todo.Title)

	buf, hasField = appendJSONFieldPrefix(buf, "description", hasField)
	buf = appendJSONString(buf, todo.Description)

	buf, hasField = appendJSONFieldPrefix(buf, "status", hasField)
	buf = appendJSONString(buf, string(todo.Status))

	buf, hasField = appendJSONFieldPrefix(buf, "priority", hasField)
	buf = strconv.AppendInt(buf, int64(todo.Priority), 10)

	buf, hasField = appendJSONFieldPrefix(buf, "type", hasField)
	buf = appendJSONString(buf, string(todo.Type))

	if todo.ImplementationModel != "" {
		buf, hasField = appendJSONFieldPrefix(buf, "implementation_model", hasField)
		buf = appendJSONString(buf, todo.ImplementationModel)
	}
	if todo.CodeReviewModel != "" {
		buf, hasField = appendJSONFieldPrefix(buf, "code_review_model", hasField)
		buf = appendJSONString(buf, todo.CodeReviewModel)
	}
	if todo.ProjectReviewModel != "" {
		buf, hasField = appendJSONFieldPrefix(buf, "project_review_model", hasField)
		buf = appendJSONString(buf, todo.ProjectReviewModel)
	}

	buf, hasField = appendJSONFieldPrefix(buf, "created_at", hasField)
	buf = appendJSONTime(buf, todo.CreatedAt)

	buf, hasField = appendJSONFieldPrefix(buf, "updated_at", hasField)
	buf = appendJSONTime(buf, todo.UpdatedAt)

	buf, hasField = appendOptionalJSONTime(buf, "closed_at", todo.ClosedAt, hasField)
	buf, hasField = appendOptionalJSONTime(buf, "started_at", todo.StartedAt, hasField)
	buf, hasField = appendOptionalJSONTime(buf, "completed_at", todo.CompletedAt, hasField)
	if todo.JobID != "" {
		buf, hasField = appendJSONFieldPrefix(buf, "job_id", hasField)
		buf = appendJSONString(buf, todo.JobID)
	}
	buf, hasField = appendOptionalJSONTime(buf, "deleted_at", todo.DeletedAt, hasField)
	if todo.DeleteReason != "" {
		buf, hasField = appendJSONFieldPrefix(buf, "delete_reason", hasField)
		buf = appendJSONString(buf, todo.DeleteReason)
	}
	if todo.Source != "" {
		buf, _ = appendJSONFieldPrefix(buf, "source", hasField)
		buf = appendJSONString(buf, todo.Source)
	}

	buf = append(buf, '}', '\n')
	return buf
}

func appendDependencyJSONLine(buf []byte, dependency *Dependency) []byte {
	buf = append(buf, '{')
	hasField := false

	buf, hasField = appendJSONFieldPrefix(buf, "todo_id", hasField)
	buf = appendJSONString(buf, dependency.TodoID)

	buf, hasField = appendJSONFieldPrefix(buf, "depends_on_id", hasField)
	buf = appendJSONString(buf, dependency.DependsOnID)

	buf, _ = appendJSONFieldPrefix(buf, "created_at", hasField)
	buf = appendJSONTime(buf, dependency.CreatedAt)

	buf = append(buf, '}', '\n')
	return buf
}

func appendJSONFieldPrefix(buf []byte, key string, hasField bool) ([]byte, bool) {
	if hasField {
		buf = append(buf, ',')
	}
	buf = append(buf, '"')
	buf = append(buf, key...)
	buf = append(buf, '"', ':')
	return buf, true
}

func appendOptionalJSONTime(buf []byte, key string, value *time.Time, hasField bool) ([]byte, bool) {
	if value == nil {
		return buf, hasField
	}
	buf, hasField = appendJSONFieldPrefix(buf, key, hasField)
	buf = appendJSONTime(buf, *value)
	return buf, hasField
}

func appendJSONString(buf []byte, value string) []byte {
	buf = append(buf, '"')
	for i := 0; i < len(value); i++ {
		char := value[i]
		switch char {
		case '\\', '"':
			buf = append(buf, '\\', char)
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\r':
			buf = append(buf, '\\', 'r')
		case '\t':
			buf = append(buf, '\\', 't')
		case '\b':
			buf = append(buf, '\\', 'b')
		case '\f':
			buf = append(buf, '\\', 'f')
		default:
			if char < 0x20 {
				buf = append(buf, '\\', 'u', '0', '0', jsonHexDigits[char>>4], jsonHexDigits[char&0x0f])
			} else {
				buf = append(buf, char)
			}
		}
	}
	buf = append(buf, '"')
	return buf
}

func appendJSONTime(buf []byte, value time.Time) []byte {
	buf = append(buf, '"')
	buf = value.AppendFormat(buf, time.RFC3339Nano)
	buf = append(buf, '"')
	return buf
}

func readJSONLStore[T any](store *Store, filename string) ([]T, error) {
	var items []T
	found, err := withStoreReader(store, filename, func(reader io.Reader) error {
		var err error
		items, err = readJSONLFromReader[T](reader)
		return err
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return items, nil
}

func readJSONLStoreWithContext[T any](store *Store, filename, label string) ([]T, error) {
	items, err := readJSONLStore[T](store, filename)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	return items, nil
}

func (s *Store) readTodosByExactIDs(missing map[string]struct{}) (map[string]Todo, error) {
	if len(missing) == 0 {
		return nil, nil
	}
	var items map[string]Todo
	found, err := withStoreReader(s, TodosFile, func(reader io.Reader) error {
		var err error
		items, err = readTodosByExactIDsFromReader(reader, missing)
		if err != nil {
			return fmt.Errorf("read todos: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return items, nil
}

func withStoreReader(store *Store, filename string, fn func(io.Reader) error) (bool, error) {
	if store.readOnly {
		output, err := readBookmarkFile(store.client, store.repoPath, filename)
		if err != nil {
			return false, err
		}
		if output == nil {
			return false, nil
		}
		return true, fn(bytes.NewReader(output))
	}

	path := storeFilePath(store.wsPath, filename)
	err := withFileLock(path, func(file *os.File) error {
		return fn(file)
	})
	return true, err
}

func (s *Store) readTodosWithContext() ([]Todo, error) {
	return readJSONLStoreWithContext[Todo](s, TodosFile, "todos")
}

// IDIndex returns an index of all todo IDs in the store.
func (s *Store) IDIndex() (IDIndex, error) {
	todos, err := s.readTodosWithContext()
	if err != nil {
		return IDIndex{}, err
	}
	return NewIDIndex(todos), nil
}

func writeJSONLStore[T any](store *Store, filename string, items []T) error {
	if err := store.ensureWritable(); err != nil {
		return err
	}

	path := storeFilePath(store.wsPath, filename)
	err := withFileLock(path, func(*os.File) error {
		return writeJSONL(path, items)
	})
	if err != nil {
		return err
	}

	if store.snapshot == nil {
		return fmt.Errorf("snapshotter is not configured")
	}

	return snapshotStore(store)
}

func snapshotStore(store *Store) error {
	if store.snapshot == nil {
		return fmt.Errorf("snapshotter is not configured")
	}
	if err := store.snapshot.Snapshot(store.wsPath); err == nil {
		return nil
	} else if !isStaleWorkspaceError(err) {
		return err
	}
	if store.client == nil {
		return fmt.Errorf("update stale workspace: client not configured")
	}
	if err := store.client.WorkspaceUpdateStale(store.wsPath); err != nil {
		return err
	}
	return store.snapshot.Snapshot(store.wsPath)
}

func acquireTodoLock(repoPath string) (*os.File, error) {
	stateDir, err := paths.DefaultStateDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create todo lock dir: %w", err)
	}
	lockName := fmt.Sprintf("todo-%s.lock", db.SanitizeRepoName(repoPath))
	lockPath := filepath.Join(stateDir, lockName)
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open todo lock file: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("lock todo store: %w", err)
	}
	return file, nil
}

func releaseTodoLock(file *os.File) error {
	if file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	closeErr := file.Close()
	// Do NOT remove the lock file. Flock operates on inodes, not paths.
	// Removing the path allows a new opener to create a different inode
	// and acquire a "lock" that doesn't conflict with any existing holder,
	// breaking mutual exclusion.
	return errors.Join(unlockErr, closeErr)
}

func isStaleWorkspaceError(err error) bool {
	if err == nil {
		return false
	}
	return internalstrings.ContainsAnyLower(err.Error(), "working copy is stale", "workspace is stale")
}

func writeJSONLStoreWithContext[T any](store *Store, filename, label string, items []T) error {
	if err := writeJSONLStore(store, filename, items); err != nil {
		return fmt.Errorf("write %s: %w", label, err)
	}
	return nil
}

// writeTodos writes all todos to the store and runs jj snapshot.
func (s *Store) writeTodos(todos []Todo) error {
	return writeJSONLStoreWithContext(s, TodosFile, "todos", todos)
}

func (s *Store) readDependenciesWithContext() ([]Dependency, error) {
	return readJSONLStoreWithContext[Dependency](s, DependenciesFile, "dependencies")
}

// writeDependencies writes all dependencies to the store and runs jj snapshot.
func (s *Store) writeDependencies(deps []Dependency) error {
	return writeJSONLStoreWithContext(s, DependenciesFile, "dependencies", deps)
}

func (s *Store) resolveTodoIDs(ids []string) ([]string, error) {
	_, resolved, err := s.readTodosAndResolveIDs(ids)
	return resolved, err
}

func (s *Store) readTodosAndResolveIDs(ids []string) ([]Todo, []string, error) {
	todos, err := s.readTodosWithContext()
	if err != nil {
		return nil, nil, err
	}

	resolved, err := resolveTodoIDsWithTodos(ids, todos)
	if err != nil {
		return nil, nil, err
	}

	return todos, resolved, nil
}

func resolveTodoIDsWithTodos(ids []string, todos []Todo) ([]string, error) {
	if err := validateTodoIDs(ids); err != nil {
		return nil, err
	}
	if resolved, ok, err := resolveExactTodoIDs(ids, todos); err != nil {
		return nil, err
	} else if ok {
		return resolved, nil
	}

	index := NewIDIndex(todos)
	resolved := make([]string, 0, len(ids))
	for _, id := range ids {
		resolvedID, err := index.Resolve(id)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, resolvedID)
	}

	return resolved, nil
}

func resolveExactTodoIDs(ids []string, todos []Todo) ([]string, bool, error) {
	missing, ok := exactTodoIDSet(ids)
	if !ok {
		return nil, false, nil
	}
	for _, todo := range todos {
		if _, ok := missing[todo.ID]; ok {
			delete(missing, todo.ID)
			if len(missing) == 0 {
				break
			}
		}
	}
	if len(missing) > 0 {
		return nil, true, ErrTodoNotFound
	}
	return ids, true, nil
}

func exactTodoIDSet(ids []string) (map[string]struct{}, bool) {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if len(id) != internalids.DefaultLength || !isNormalizedID(id) {
			return nil, false
		}
		set[id] = struct{}{}
	}
	return set, true
}

func isNormalizedID(id string) bool {
	for i := 0; i < len(id); i++ {
		b := id[i]
		if b >= utf8.RuneSelf || (b >= 'A' && b <= 'Z') {
			return false
		}
	}
	return true
}

func validateTodoIDs(ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("no todo IDs provided")
	}
	return nil
}

func readBookmarkFile(client *jj.Client, repoPath, path string) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("todo store is missing jj client")
	}
	output, err := client.FileShow(repoPath, BookmarkName, path)
	if errors.Is(err, jj.ErrFileNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return output, nil
}

func (s *Store) ensureWritable() error {
	if s.readOnly {
		return ErrReadOnlyStore
	}
	if s.wsPath == "" {
		return fmt.Errorf("todo store workspace is not available")
	}
	return nil
}
