package testsupport

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// EnsureHomeDirs creates the default state and workspace directories under homeDir.
func EnsureHomeDirs(homeDir string) error {
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incrementum"), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incrementum", "workspaces"), 0o755); err != nil {
		return fmt.Errorf("create workspaces dir: %w", err)
	}
	return nil
}

// SetupTestHome creates a temp home directory, ensures state/workspace dirs, and sets HOME.
func SetupTestHome(t testing.TB) string {
	t.Helper()

	homeDir := t.TempDir()
	if err := EnsureHomeDirs(homeDir); err != nil {
		t.Fatalf("setup home dir: %v", err)
	}
	t.Setenv("HOME", homeDir)
	return homeDir
}
