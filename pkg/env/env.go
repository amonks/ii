package env

import (
	"os"
	"path/filepath"
)

func InMonksRoot(path ...string) string {
	root := os.Getenv("MONKS_ROOT")
	if root == "" {
		panic("MONKS_ROOT environment variable is not set")
	}
	parts := append([]string{root}, path...)
	return filepath.Join(parts...)
}

func InMonksData(path ...string) string {
	data := os.Getenv("MONKS_DATA")
	if data == "" {
		panic("MONKS_DATA environment variable is not set")
	}
	parts := append([]string{data}, path...)
	return filepath.Join(parts...)
}
