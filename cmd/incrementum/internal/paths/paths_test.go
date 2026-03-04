package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultStateDirUsesHome(t *testing.T) {
	t.Setenv("HOME", filepath.Join("/tmp", "test-home"))

	dir, err := DefaultStateDir()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := filepath.Join("/tmp", "test-home", ".local", "state", "incrementum")
	if dir != expected {
		t.Fatalf("expected %s, got %s", expected, dir)
	}
}

func TestHomeDirUsesHome(t *testing.T) {
	t.Setenv("HOME", filepath.Join("/tmp", "test-home"))

	home, err := HomeDir()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if home != filepath.Join("/tmp", "test-home") {
		t.Fatalf("expected %s, got %s", filepath.Join("/tmp", "test-home"), home)
	}
}

func TestDefaultWorkspacesDirUsesHome(t *testing.T) {
	t.Setenv("HOME", filepath.Join("/tmp", "test-home"))

	dir, err := DefaultWorkspacesDir()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := filepath.Join("/tmp", "test-home", ".local", "share", "incrementum", "workspaces")
	if dir != expected {
		t.Fatalf("expected %s, got %s", expected, dir)
	}
}

func TestDefaultDBPathUsesHome(t *testing.T) {
	t.Setenv("HOME", filepath.Join("/tmp", "test-home"))

	path, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := filepath.Join("/tmp", "test-home", ".local", "state", "incrementum", "state.db")
	if path != expected {
		t.Fatalf("expected %s, got %s", expected, path)
	}
}

func TestWorkingDirReturnsCurrentDir(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	workDir := t.TempDir()
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	resolved, err := WorkingDir()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resolved != workDir {
		t.Fatalf("expected %s, got %s", workDir, resolved)
	}
}

func TestResolveWithDefault(t *testing.T) {
	t.Run("returns override when provided", func(t *testing.T) {
		result, err := ResolveWithDefault("/custom/path", DefaultStateDir)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result != "/custom/path" {
			t.Fatalf("expected /custom/path, got %s", result)
		}
	})

	t.Run("calls default function when override is empty", func(t *testing.T) {
		t.Setenv("HOME", filepath.Join("/tmp", "test-home"))

		result, err := ResolveWithDefault("", DefaultStateDir)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expected := filepath.Join("/tmp", "test-home", ".local", "state", "incrementum")
		if result != expected {
			t.Fatalf("expected %s, got %s", expected, result)
		}
	})

	t.Run("propagates error from default function", func(t *testing.T) {
		errorFn := func() (string, error) {
			return "", os.ErrNotExist
		}

		_, err := ResolveWithDefault("", errorFn)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err != os.ErrNotExist {
			t.Fatalf("expected os.ErrNotExist, got %v", err)
		}
	})
}
