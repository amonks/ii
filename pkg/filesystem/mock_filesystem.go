package filesystem

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MockEntry represents a file or directory in the mock filesystem
type MockEntry struct {
	Name     string
	IsDir    bool
	Size     int64
	ModTime  time.Time
	Contents []byte
	Children map[string]*MockEntry
}

// MockFS implements FS with an in-memory filesystem
type MockFS struct {
	Root *MockEntry
}

// NewMockFS creates a new mock filesystem
func NewMockFS() *MockFS {
	return &MockFS{
		Root: &MockEntry{
			Name:     "/",
			IsDir:    true,
			Children: make(map[string]*MockEntry),
		},
	}
}

// AddDir adds a directory to the mock filesystem
func (fs *MockFS) AddDir(path string) error {
	parts := splitPath(path)
	entry := fs.Root

	// Create parent directories if needed
	for i, part := range parts {
		if i == len(parts)-1 {
			// Create target directory
			entry.Children[part] = &MockEntry{
				Name:     part,
				IsDir:    true,
				Children: make(map[string]*MockEntry),
				ModTime:  time.Now(),
			}
			return nil
		}

		child, exists := entry.Children[part]
		if !exists {
			// Create intermediate directory
			child = &MockEntry{
				Name:     part,
				IsDir:    true,
				Children: make(map[string]*MockEntry),
				ModTime:  time.Now(),
			}
			entry.Children[part] = child
		}
		entry = child
	}
	return nil
}

// AddFile adds a file to the mock filesystem
func (fs *MockFS) AddFile(path string, contents []byte) error {
	parts := splitPath(path)
	if len(parts) == 0 {
		return fmt.Errorf("invalid path")
	}

	entry := fs.Root
	// Navigate to parent directory
	for i, part := range parts {
		if i == len(parts)-1 {
			// Create the file
			entry.Children[part] = &MockEntry{
				Name:     part,
				IsDir:    false,
				Size:     int64(len(contents)),
				Contents: contents,
				ModTime:  time.Now(),
			}
			return nil
		}

		child, exists := entry.Children[part]
		if !exists {
			// Create intermediate directory
			child = &MockEntry{
				Name:     part,
				IsDir:    true,
				Children: make(map[string]*MockEntry),
				ModTime:  time.Now(),
			}
			entry.Children[part] = child
		}
		entry = child
	}
	return nil
}

// GetEntry finds an entry at the given path
func (fs *MockFS) GetEntry(path string) (*MockEntry, error) {
	// Root path
	if path == "/" || path == "" {
		return fs.Root, nil
	}

	parts := splitPath(path)
	entry := fs.Root

	for _, part := range parts {
		child, exists := entry.Children[part]
		if !exists {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		entry = child
	}
	return entry, nil
}

// ReadDir reads a directory in the mock filesystem
func (fs *MockFS) ReadDir(path string) ([]os.DirEntry, error) {
	entry, err := fs.GetEntry(path)
	if err != nil {
		return nil, err
	}

	if !entry.IsDir {
		return nil, fmt.Errorf("not a directory: %s", path)
	}

	dirEntries := make([]os.DirEntry, 0, len(entry.Children))
	for _, child := range entry.Children {
		dirEntries = append(dirEntries, &mockDirEntry{
			name:  child.Name,
			isDir: child.IsDir,
		})
	}
	return dirEntries, nil
}

// Stat returns file info about a path
func (fs *MockFS) Stat(path string) (os.FileInfo, error) {
	entry, err := fs.GetEntry(path)
	if err != nil {
		return nil, err
	}
	return &mockFileInfo{
		name:    entry.Name,
		size:    entry.Size,
		mode:    modeForEntry(entry),
		modTime: entry.ModTime,
		isDir:   entry.IsDir,
	}, nil
}

// Open opens a file for reading
func (fs *MockFS) Open(path string) (*os.File, error) {
	// This is a mock, so we can't return a real *os.File
	// Instead, we'll throw an error indicating this is not supported
	return nil, fmt.Errorf("MockFS.Open not implemented - use ReadFile for testing")
}

// Create creates a file for writing
func (fs *MockFS) Create(path string) (*os.File, error) {
	// Similar to Open, we can't return a real file
	return nil, fmt.Errorf("MockFS.Create not implemented - use WriteFile for testing")
}

// MkdirAll creates directories in the mock filesystem
func (fs *MockFS) MkdirAll(path string, perm os.FileMode) error {
	return fs.AddDir(path)
}

// Join joins path elements
func (fs *MockFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Copy copies data between readers and writers
func (fs *MockFS) Copy(src io.Reader, dst io.Writer) (int64, error) {
	return io.Copy(dst, src)
}

// Helper methods for the mock filesystem

// ReadFile reads a file's contents
func (fs *MockFS) ReadFile(path string) ([]byte, error) {
	entry, err := fs.GetEntry(path)
	if err != nil {
		return nil, err
	}
	if entry.IsDir {
		return nil, fmt.Errorf("cannot read directory: %s", path)
	}
	return entry.Contents, nil
}

// WriteFile writes contents to a file
func (fs *MockFS) WriteFile(path string, contents []byte) error {
	return fs.AddFile(path, contents)
}

// Helper function to split path into parts
func splitPath(path string) []string {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}

// Helper function to determine file mode
func modeForEntry(entry *MockEntry) os.FileMode {
	if entry.IsDir {
		return os.ModeDir | 0755
	}
	return 0644
}

// Mock implementations of OS interfaces

// mockDirEntry implements os.DirEntry
type mockDirEntry struct {
	name  string
	isDir bool
}

func (e *mockDirEntry) Name() string               { return e.name }
func (e *mockDirEntry) IsDir() bool                { return e.isDir }
func (e *mockDirEntry) Type() fs.FileMode          { return modeForType(e.isDir) }
func (e *mockDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func modeForType(isDir bool) fs.FileMode {
	if isDir {
		return os.ModeDir | 0755
	}
	return 0644
}

// mockFileInfo implements os.FileInfo
type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *mockFileInfo) Name() string       { return fi.name }
func (fi *mockFileInfo) Size() int64        { return fi.size }
func (fi *mockFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *mockFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *mockFileInfo) IsDir() bool        { return fi.isDir }
func (fi *mockFileInfo) Sys() interface{}   { return nil }