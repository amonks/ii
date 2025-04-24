package filesystem

import (
	"io"
	"os"
	"path/filepath"
)

// FS abstracts filesystem operations for testability
type FS interface {
	ReadDir(path string) ([]os.DirEntry, error)
	Stat(path string) (os.FileInfo, error)
	Open(path string) (*os.File, error)
	Create(path string) (*os.File, error)
	MkdirAll(path string, perm os.FileMode) error
	Join(elem ...string) string
	Copy(src io.Reader, dst io.Writer) (int64, error)
}

// OSFileSystem implements FS with real OS calls
type OSFileSystem struct{}

func NewOSFileSystem() *OSFileSystem {
	return &OSFileSystem{}
}

func (fs *OSFileSystem) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func (fs *OSFileSystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (fs *OSFileSystem) Open(path string) (*os.File, error) {
	return os.Open(path)
}

func (fs *OSFileSystem) Create(path string) (*os.File, error) {
	return os.Create(path)
}

func (fs *OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (fs *OSFileSystem) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *OSFileSystem) Copy(src io.Reader, dst io.Writer) (int64, error) {
	return io.Copy(dst, src)
}