package env

import (
	"os"
	"path/filepath"
)

func InMonksRoot(path ...string) string {
	parts := append([]string{os.Getenv("MONKS_ROOT")}, path...)
	return filepath.Join(parts...)
}

func InMonksData(path ...string) string {
	parts := append([]string{os.Getenv("MONKS_DATA")}, path...)
	return filepath.Join(parts...)
}
