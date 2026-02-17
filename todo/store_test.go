package todo

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/jj"
	"github.com/creack/pty"
)

// setupTestRepo creates a temporary jj repository for testing.
// It also sets HOME to a temp directory to prevent leaking state into
// ~/.local/state/incrementum and ~/.local/share/incrementum/workspaces.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var)
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	// Set HOME to a temp directory to isolate state/workspaces
	homeDir := t.TempDir()
	os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incrementum"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incrementum", "workspaces"), 0755)
	t.Setenv("HOME", homeDir)

	client := jj.New()
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	return tmpDir
}

func setTTYInput(t *testing.T, input string) {
	t.Helper()

	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("open tty: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = slave
	if _, err := master.Write([]byte(input)); err != nil {
		_ = master.Close()
		_ = slave.Close()
		os.Stdin = oldStdin
		t.Fatalf("write tty input: %v", err)
	}
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = master.Close()
		_ = slave.Close()
	})
}

func TestOpen_NoBookmark(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Try to open without CreateIfMissing - should fail
	_, err := Open(repoPath, OpenOptions{
		CreateIfMissing: false,
	})
	if !errors.Is(err, ErrNoTodoStore) {
		t.Errorf("expected ErrNoTodoStore, got %v", err)
	}
}

func TestOpen_CreateIfMissing(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Open with CreateIfMissing
	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Verify the bookmark was created
	client := jj.New()
	bookmarks, err := client.BookmarkList(repoPath)
	if err != nil {
		t.Fatalf("failed to list bookmarks: %v", err)
	}

	found := slices.Contains(bookmarks, BookmarkName)
	if !found {
		t.Errorf("bookmark %q not found in %v", BookmarkName, bookmarks)
	}
}

func TestOpen_UsesPurpose(t *testing.T) {
	repoPath := setupTestRepo(t)

	purpose := "todo store (purpose test)"
	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
		Purpose:         purpose,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	infos, err := store.pool.List(repoPath)
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}

	for _, info := range infos {
		if info.Path != store.wsPath {
			continue
		}
		if info.Purpose != purpose {
			t.Fatalf("expected purpose %q, got %q", purpose, info.Purpose)
		}
		return
	}

	t.Fatalf("workspace for store not found")
}

func TestOpen_PromptToCreate_Confirmed(t *testing.T) {
	repoPath := setupTestRepo(t)
	setTTYInput(t, "y\n")

	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  true,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()
}

func TestOpen_PromptToCreate_Declined(t *testing.T) {
	repoPath := setupTestRepo(t)
	setTTYInput(t, "n\n")

	_, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  true,
	})
	if !errors.Is(err, ErrNoTodoStore) {
		t.Errorf("expected ErrNoTodoStore, got %v", err)
	}
}

func TestOpen_PromptToCreate_NonTTY(t *testing.T) {
	repoPath := setupTestRepo(t)

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = oldStdin
		reader.Close()
		writer.Close()
	})

	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  true,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	client := jj.New()
	bookmarks, err := client.BookmarkList(repoPath)
	if err != nil {
		t.Fatalf("failed to list bookmarks: %v", err)
	}

	found := slices.Contains(bookmarks, BookmarkName)
	if !found {
		t.Errorf("bookmark %q not found in %v", BookmarkName, bookmarks)
	}
}

func TestOpen_ExistingStore(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Create the store first
	store1, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	store1.Release()

	// Open it again - should work without creating
	store2, err := Open(repoPath, OpenOptions{
		CreateIfMissing: false,
	})
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}
	defer store2.Release()
}

func TestOpen_ReadOnlyDoesNotAcquireWorkspace(t *testing.T) {
	repoPath := setupTestRepo(t)

	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}

	_, err = store.Create("Test todo", CreateOptions{})
	if err != nil {
		store.Release()
		t.Fatalf("failed to create todo: %v", err)
	}
	store.Release()

	readStore, err := Open(repoPath, OpenOptions{
		ReadOnly: true,
	})
	if err != nil {
		t.Fatalf("failed to open read-only store: %v", err)
	}
	defer readStore.Release()

	if readStore.wsPath != "" {
		t.Fatalf("expected read-only store to have empty workspace path, got %q", readStore.wsPath)
	}
	if readStore.pool != nil {
		t.Fatalf("expected read-only store to not have a workspace pool")
	}

	todos, err := readStore.List(ListFilter{})
	if err != nil {
		t.Fatalf("failed to list todos: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}
	if todos[0].Title != "Test todo" {
		t.Fatalf("expected todo title %q, got %q", "Test todo", todos[0].Title)
	}
}

func TestStore_ReadWriteTodos(t *testing.T) {
	store := newTestStore(t)

	// Initially empty
	todos, err := store.readTodos()
	if err != nil {
		t.Fatalf("failed to read todos: %v", err)
	}
	if len(todos) != 0 {
		t.Errorf("expected 0 todos, got %d", len(todos))
	}

	// Write some todos
	now := time.Now()
	testTodos := []Todo{
		{
			ID:        "abc12345",
			Title:     "Test todo 1",
			Status:    StatusOpen,
			Priority:  PriorityMedium,
			Type:      TypeTask,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "def67890",
			Title:     "Test todo 2",
			Status:    StatusInProgress,
			Priority:  PriorityHigh,
			Type:      TypeBug,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := store.writeTodos(testTodos); err != nil {
		t.Fatalf("failed to write todos: %v", err)
	}

	// Read them back
	todos, err = store.readTodos()
	if err != nil {
		t.Fatalf("failed to read todos: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(todos))
	}

	if todos[0].ID != "abc12345" {
		t.Errorf("expected ID 'abc12345', got %q", todos[0].ID)
	}
	if todos[1].Title != "Test todo 2" {
		t.Errorf("expected title 'Test todo 2', got %q", todos[1].Title)
	}
}

func TestStore_SnapshotsOnWriteTodos(t *testing.T) {
	store := &Store{
		repoPath: "test",
		wsPath:   t.TempDir(),
		wsRelease: func() error {
			return nil
		},
	}

	var calls int
	store.snapshot = snapshotterFunc(func(workspacePath string) error {
		calls++
		return nil
	})

	now := time.Now()
	input := []Todo{{
		ID:        "abc12345",
		Title:     "Test todo",
		Status:    StatusOpen,
		Priority:  PriorityMedium,
		Type:      TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}}

	if err := store.writeTodos(input); err != nil {
		t.Fatalf("failed to write todos: %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected 1 snapshot call, got %d", calls)
	}
}

func TestStore_ReadWriteTodos_LongDescription(t *testing.T) {
	store := newTestStore(t)

	longDescription := strings.Repeat("a", 100000)
	now := time.Now()
	input := []Todo{
		{
			ID:          "longdesc",
			Title:       "Long description todo",
			Description: longDescription,
			Status:      StatusOpen,
			Priority:    PriorityMedium,
			Type:        TypeTask,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	if err := store.writeTodos(input); err != nil {
		t.Fatalf("failed to write todos: %v", err)
	}

	output, err := store.readTodos()
	if err != nil {
		t.Fatalf("failed to read todos: %v", err)
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(output))
	}
	if output[0].Description != longDescription {
		t.Errorf("description mismatch: expected %d bytes, got %d", len(longDescription), len(output[0].Description))
	}
}

func TestStore_ReadWriteDependencies(t *testing.T) {
	store := newTestStore(t)

	// Initially empty
	deps, err := store.readDependencies()
	if err != nil {
		t.Fatalf("failed to read dependencies: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(deps))
	}

	// Write some dependencies
	now := time.Now()
	testDeps := []Dependency{
		{
			TodoID:      "abc12345",
			DependsOnID: "def67890",
			CreatedAt:   now,
		},
	}

	if err := store.writeDependencies(testDeps); err != nil {
		t.Fatalf("failed to write dependencies: %v", err)
	}

	// Read them back
	deps, err = store.readDependencies()
	if err != nil {
		t.Fatalf("failed to read dependencies: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}

	if deps[0].TodoID != "abc12345" {
		t.Errorf("expected TodoID 'abc12345', got %q", deps[0].TodoID)
	}
}

func TestStore_GetTodoByID(t *testing.T) {
	store := newTestStore(t)

	now := time.Now()
	testTodos := []Todo{
		{
			ID:        "abc12345",
			Title:     "Test todo 1",
			Status:    StatusOpen,
			Priority:  PriorityMedium,
			Type:      TypeTask,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := store.writeTodos(testTodos); err != nil {
		t.Fatalf("failed to write todos: %v", err)
	}

	// Find existing todo
	todo, err := store.getTodoByID("abc12345")
	if err != nil {
		t.Fatalf("failed to get todo: %v", err)
	}
	if todo.Title != "Test todo 1" {
		t.Errorf("expected title 'Test todo 1', got %q", todo.Title)
	}

	// Try to find non-existent todo
	_, err = store.getTodoByID("nonexistent")
	if !errors.Is(err, ErrTodoNotFound) {
		t.Errorf("expected ErrTodoNotFound, got %v", err)
	}
}

func TestReadJSONL_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	// Non-existent file should return empty slice
	items, err := readJSONL[Todo](path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty slice, got %d items", len(items))
	}

	// Empty file should return empty slice
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	items, err = readJSONL[Todo](path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty slice, got %d items", len(items))
	}
}

func TestReadJSONLFromReader_MaxLineSize(t *testing.T) {
	line := bytes.Repeat([]byte("a"), maxJSONLineBytes+1)
	data := append(line, '\n')

	_, err := readJSONLFromReader[Todo](bytes.NewReader(data))
	if err == nil || !strings.Contains(err.Error(), "exceeds max JSON line size") {
		t.Fatalf("expected max line size error, got %v", err)
	}
}

func TestReadJSONLFromReader_MultilineJSON(t *testing.T) {
	data := []byte("{\"id\":\"abc12345\",\n\"title\":\"oops\"}\n")

	_, err := readJSONLFromReader[Todo](bytes.NewReader(data))
	if err == nil {
		t.Fatalf("expected multiline JSON to return an error")
	}
}

func TestReadJSONLFromReader_SkipsBlankLines(t *testing.T) {
	data := []byte("{\"id\":\"abc12345\",\"title\":\"First\"}\n\n\r\n{\"id\":\"def67890\",\"title\":\"Second\"}\n\n")

	items, err := readJSONLFromReader[Todo](bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "abc12345" {
		t.Errorf("expected first item ID to be abc12345, got %q", items[0].ID)
	}
	if items[1].ID != "def67890" {
		t.Errorf("expected second item ID to be def67890, got %q", items[1].ID)
	}
}

func TestWriteJSONL_Atomic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	now := time.Now()
	todos := []Todo{
		{
			ID:        "abc12345",
			Title:     "Test",
			Status:    StatusOpen,
			Priority:  PriorityMedium,
			Type:      TypeTask,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := writeJSONL(path, todos); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Verify no temp file left behind
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file was not cleaned up")
	}

	// Verify content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if len(data) == 0 {
		t.Error("file is empty")
	}
}

func TestWriteJSONL_RoundTripTodos(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "todos.jsonl")

	baseTime := time.Date(2026, time.January, 25, 8, 0, 0, 123456000, time.UTC)
	closedAt := baseTime.Add(2 * time.Hour)
	startedAt := baseTime.Add(30 * time.Minute)
	completedAt := baseTime.Add(3 * time.Hour)
	deletedAt := baseTime.Add(4 * time.Hour)

	todos := []Todo{
		{
			ID:          "abc12345",
			Title:       "Needs \"quotes\"",
			Description: "Line\nwith break",
			Status:      StatusOpen,
			Priority:    PriorityMedium,
			Type:        TypeTask,
			CreatedAt:   baseTime,
			UpdatedAt:   baseTime,
		},
		{
			ID:           "def67890",
			Title:        "Second",
			Description:  "",
			Status:       StatusClosed,
			Priority:     PriorityHigh,
			Type:         TypeBug,
			CreatedAt:    baseTime,
			UpdatedAt:    baseTime,
			ClosedAt:     &closedAt,
			StartedAt:    &startedAt,
			CompletedAt:  &completedAt,
			DeletedAt:    &deletedAt,
			DeleteReason: "all done",
			Source:       "habit:cleanup",
		},
	}

	if err := writeJSONL(path, todos); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	loaded, err := readJSONL[Todo](path)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if len(loaded) != len(todos) {
		t.Fatalf("expected %d todos, got %d", len(todos), len(loaded))
	}
	for i := range todos {
		assertTodoEqual(t, loaded[i], todos[i])
	}
}

func TestWriteJSONL_RoundTripDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "dependencies.jsonl")

	createdAt := time.Date(2026, time.January, 25, 9, 30, 0, 0, time.UTC)
	deps := []Dependency{
		{
			TodoID:      "abc12345",
			DependsOnID: "def67890",
			CreatedAt:   createdAt,
		},
	}

	if err := writeJSONL(path, deps); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	loaded, err := readJSONL[Dependency](path)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if len(loaded) != len(deps) {
		t.Fatalf("expected %d deps, got %d", len(deps), len(loaded))
	}
	if loaded[0].TodoID != deps[0].TodoID || loaded[0].DependsOnID != deps[0].DependsOnID {
		t.Fatalf("dependency mismatch: %+v", loaded[0])
	}
	if !loaded[0].CreatedAt.Equal(deps[0].CreatedAt) {
		t.Fatalf("expected created_at %v, got %v", deps[0].CreatedAt, loaded[0].CreatedAt)
	}
}

func assertTodoEqual(t *testing.T, got, want Todo) {
	t.Helper()
	if got.ID != want.ID ||
		got.Title != want.Title ||
		got.Description != want.Description ||
		got.Status != want.Status ||
		got.Priority != want.Priority ||
		got.Type != want.Type ||
		got.DeleteReason != want.DeleteReason ||
		got.Source != want.Source {
		t.Fatalf("todo mismatch: %+v", got)
	}
	assertTimeEqual(t, "created_at", got.CreatedAt, want.CreatedAt)
	assertTimeEqual(t, "updated_at", got.UpdatedAt, want.UpdatedAt)
	assertTimePointerEqual(t, "closed_at", got.ClosedAt, want.ClosedAt)
	assertTimePointerEqual(t, "started_at", got.StartedAt, want.StartedAt)
	assertTimePointerEqual(t, "completed_at", got.CompletedAt, want.CompletedAt)
	assertTimePointerEqual(t, "deleted_at", got.DeletedAt, want.DeletedAt)
}

func assertTimeEqual(t *testing.T, field string, got, want time.Time) {
	t.Helper()
	if !got.Equal(want) {
		t.Fatalf("expected %s %v, got %v", field, want, got)
	}
}

func assertTimePointerEqual(t *testing.T, field string, got, want *time.Time) {
	t.Helper()
	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("expected %s %v, got %v", field, want, got)
	}
	if !got.Equal(*want) {
		t.Fatalf("expected %s %v, got %v", field, want, got)
	}
}

// TestOpen_DoesNotModifyUserWorkingCopy verifies that opening/creating a store
// does not modify the user's working copy. This is a regression test for a bug
// where createTaskStore would run jj commands in the user's directory, causing
// the user's @ to temporarily move to the todo store change.
func TestOpen_DoesNotModifyUserWorkingCopy(t *testing.T) {
	repoPath := setupTestRepo(t)

	client := jj.New()

	// Record the user's current change ID before opening the store
	changeIDBefore, err := client.CurrentChangeID(repoPath)
	if err != nil {
		t.Fatalf("failed to get initial change ID: %v", err)
	}

	// Open the store (which will create the todo store since it doesn't exist)
	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// The user's working copy should still be at the same change
	changeIDAfter, err := client.CurrentChangeID(repoPath)
	if err != nil {
		t.Fatalf("failed to get change ID after open: %v", err)
	}

	if changeIDBefore != changeIDAfter {
		t.Errorf("user's working copy was modified during store creation\n"+
			"before: %s\n"+
			"after:  %s\n"+
			"Store creation should operate in a background workspace, not the user's directory",
			changeIDBefore, changeIDAfter)
	}
}
