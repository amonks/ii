package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/amonks/incrementum/internal/paths"
	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// ErrRepoPathNotFound indicates a workspace is tracked but missing repo info.
var ErrRepoPathNotFound = fmt.Errorf("repo source path not found")

// Store manages the state file with locking.
type Store struct {
	dir string
}

func newState() *State {
	return &State{
		Repos:         make(map[string]RepoInfo),
		Workspaces:    make(map[string]WorkspaceInfo),
		AgentSessions: make(map[string]AgentSession),
		Jobs:          make(map[string]Job),
	}
}

func ensureStateMaps(st *State) {
	if st.Repos == nil {
		st.Repos = make(map[string]RepoInfo)
	}
	if st.Workspaces == nil {
		st.Workspaces = make(map[string]WorkspaceInfo)
	}
	if st.AgentSessions == nil {
		st.AgentSessions = make(map[string]AgentSession)
	}
	if st.Jobs == nil {
		st.Jobs = make(map[string]Job)
	}
}

// NewStore creates a new state store using the given directory.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// statePath returns the path to the state file.
func (s *Store) statePath() string {
	return filepath.Join(s.dir, "state.json")
}

// lockPath returns the path to the lock file.
func (s *Store) lockPath() string {
	return filepath.Join(s.dir, "state.lock")
}

func (s *Store) ensureStateDir() error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	return nil
}

// Load reads the state from disk. Returns an empty state if the file doesn't exist.
func (s *Store) Load() (*State, error) {
	data, err := os.ReadFile(s.statePath())
	if os.IsNotExist(err) {
		return newState(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	ensureStateMaps(&st)

	return &st, nil
}

// Save writes the state to disk.
func (s *Store) Save(st *State) error {
	if err := s.ensureStateDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if existing, err := os.ReadFile(s.statePath()); err == nil {
		if bytes.Equal(existing, data) {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read state file: %w", err)
	}

	// Write atomically via temp file
	tmpFile, err := os.CreateTemp(s.dir, filepath.Base(s.statePath())+".tmp")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	name := tmpFile.Name()
	_, err = tmpFile.Write(data)
	if err1 := tmpFile.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		os.Remove(name)
		return fmt.Errorf("write temp state file: %w", err)
	}

	if err := os.Rename(name, s.statePath()); err != nil {
		os.Remove(name)
		return fmt.Errorf("rename state file: %w", err)
	}

	return nil
}

// Update atomically reads, modifies, and writes the state with file locking.
func (s *Store) Update(fn func(st *State) error) error {
	if err := s.ensureStateDir(); err != nil {
		return err
	}

	// Open lock file
	lockFile, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer lockFile.Close()

	// Acquire exclusive lock
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	// Load current state
	st, err := s.Load()
	if err != nil {
		return err
	}

	// Apply modifications
	if err := fn(st); err != nil {
		return err
	}

	// Save updated state
	return s.Save(st)
}

// RepoPathForWorkspace returns the source repo path for a workspace path.
func (s *Store) RepoPathForWorkspace(wsPath string) (string, bool, error) {
	st, err := s.Load()
	if err != nil {
		return "", false, err
	}

	// Normalize the input path for comparison to handle macOS /private symlinks
	wsPath = paths.NormalizePath(filepath.Clean(wsPath))
	for _, ws := range st.Workspaces {
		if paths.NormalizePath(filepath.Clean(ws.Path)) != wsPath {
			continue
		}
		repo, ok := st.Repos[ws.Repo]
		if !ok || repo.SourcePath == "" {
			return "", true, ErrRepoPathNotFound
		}
		return repo.SourcePath, true, nil
	}

	return "", false, nil
}

// GetOrCreateRepoName returns the repo name for the given source path,
// creating a new entry if needed. Handles collisions by appending suffixes.
// The source path is normalized (symlinks resolved) for consistent matching.
func (s *Store) GetOrCreateRepoName(sourcePath string) (string, error) {
	// Normalize path to handle symlinks (e.g., /var -> /private/var on macOS)
	sourcePath = normalizePath(sourcePath)

	var result string

	err := s.Update(func(st *State) error {
		// Check if this path already has a name
		for name, info := range st.Repos {
			if info.SourcePath == sourcePath {
				result = name
				return nil
			}
		}

		// Generate a new name
		baseName := SanitizeRepoName(sourcePath)
		name := baseName

		// Handle collisions
		suffix := 2
		for {
			if _, exists := st.Repos[name]; !exists {
				break
			}
			name = fmt.Sprintf("%s-%d", baseName, suffix)
			suffix++
		}

		st.Repos[name] = RepoInfo{SourcePath: sourcePath}
		result = name
		return nil
	})

	return result, err
}

// SanitizeRepoName converts a file path to a safe repo name.
func SanitizeRepoName(path string) string {
	// Expand ~ if present
	if strings.HasPrefix(path, "~/") {
		home, err := paths.HomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		} else {
			path = path[2:]
		}
	}

	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Convert to lowercase
	path = internalstrings.NormalizeLower(path)

	// Replace path separators and spaces with hyphens
	path = strings.ReplaceAll(path, "/", "-")
	path = strings.ReplaceAll(path, " ", "-")

	// Remove any characters that aren't alphanumeric or hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	path = reg.ReplaceAllString(path, "")

	// Collapse multiple hyphens
	reg = regexp.MustCompile(`-+`)
	path = reg.ReplaceAllString(path, "-")

	// Trim leading/trailing hyphens
	path = strings.Trim(path, "-")

	return path
}

// normalizePath resolves symlinks and returns a clean absolute path.
// This ensures consistent path matching regardless of how the path was accessed.
func normalizePath(path string) string {
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}
