package gzipserver

import (
	"bytes"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

type testFS struct {
	files map[string][]byte
}

func newTestFS() FileSystem {
	return &testFS{
		files: map[string][]byte{
			"/test.txt":       []byte("hello"),
			"/test.txt.gz":    []byte("hello-gzipped"),
			"/test.txt.br":    []byte("hello-brotli"),
			"/dir/":           nil, // directory entry
			"/dir/index.html": []byte("<html>index</html>"),
		},
	}
}

func (f *testFS) Open(name string) (http.File, error) {
	// Handle directory access
	if name == "/dir" || name == "/dir/" {
		return &testFile{
			Reader: bytes.NewReader(nil),
			name:   name,
			size:   0,
			isDir:  true,
		}, nil
	}

	data, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return &testFile{
		Reader: bytes.NewReader(data),
		name:   name,
		size:   int64(len(data)),
		isDir:  false,
	}, nil
}

func (f *testFS) Exists(name string) bool {
	if name == "/dir" || name == "/dir/" {
		return true
	}
	_, ok := f.files[name]
	return ok
}

type testFile struct {
	*bytes.Reader
	name  string
	size  int64
	isDir bool
}

func (f *testFile) Close() error                             { return nil }
func (f *testFile) Stat() (os.FileInfo, error)               { return f, nil }
func (f *testFile) Name() string                             { return f.name }
func (f *testFile) Size() int64                              { return f.size }
func (f *testFile) Mode() fs.FileMode                        { return 0644 }
func (f *testFile) ModTime() time.Time                       { return time.Now() }
func (f *testFile) IsDir() bool                              { return f.isDir }
func (f *testFile) Sys() interface{}                         { return nil }
func (f *testFile) Readdir(count int) ([]os.FileInfo, error) { return nil, fs.ErrNotExist }

func TestDirExists(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	dir := Dir(tmpDir)
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"existing file", "test.txt", true},
		{"missing file", "missing.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dir.Exists(tt.path); got != tt.want {
				t.Errorf("Dir.Exists(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFSExists(t *testing.T) {
	fsys := fstest.MapFS{
		"test.txt": &fstest.MapFile{
			Data: []byte("hello"),
		},
	}

	fs := FS(fsys)
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"existing file", "/test.txt", true},
		{"missing file", "/missing.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fs.Exists(tt.path); got != tt.want {
				t.Errorf("FS.Exists(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
