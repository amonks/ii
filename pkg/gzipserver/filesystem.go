package gzipserver

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// FileSystem is a wrapper around the http.FileSystem interface, adding a method to let us check for the existence
// of files without (attempting to) open them.
type FileSystem interface {
	http.FileSystem
	Exists(string) bool
}

// Dir is a replacement for the http.Dir type, and implements FileSystem.
type Dir string

// Exists tests whether a file with the specified name exists, resolved relative to the base directory.
func (d Dir) Exists(name string) bool {
	if filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator) {
		return false
	}
	name = path.Clean("/" + name)
	if strings.Contains(name, "\\") {
		return false
	}
	dir := string(d)
	if dir == "" {
		dir = "."
	}
	fullName := filepath.Join(dir, filepath.FromSlash(name))
	_, err := os.Stat(fullName)
	return err == nil
}

// Open defers to http.Dir's Open so that gzipped.Dir implements http.FileSystem.
func (d Dir) Open(name string) (http.File, error) {
	return http.Dir(d).Open(name)
}

func FS(f fs.FS) FileSystem {
	return &fsAdapter{fs: f}
}

type fsAdapter struct {
	fs fs.FS
}

// Exists tests whether a file with the specified name exists, resolved relative to the file system.
func (f *fsAdapter) Exists(name string) bool {
	if filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator) {
		return false
	}
	name = path.Clean("/" + name)
	if strings.Contains(name, "\\") {
		return false
	}
	_, err := fs.Stat(f.fs, strings.TrimPrefix(name, "/"))
	return err == nil
}

// Open defers to http.FS's Open so that gzipped.fsAdapter implements http.FileSystem.
func (f *fsAdapter) Open(name string) (http.File, error) {
	return http.FS(f.fs).Open(strings.TrimPrefix(name, "/"))
}
