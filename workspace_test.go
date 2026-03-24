package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"monks.co/ii/internal/db"
	"monks.co/pkg/jj"
	"monks.co/ww/ww"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	client := jj.New()
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	return tmpDir
}

func withCwd(t *testing.T, dir string, fn func()) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	fn()
}

func TestResolveWorkspaceNameFromArgs(t *testing.T) {
	stateDir := t.TempDir()
	workspacesDir := t.TempDir()
	workspacesDir, _ = filepath.EvalSymlinks(workspacesDir)

	dbPath := filepath.Join(stateDir, "state.db")
	dbStore, err := db.Open(dbPath, db.OpenOptions{LegacyJSONPath: filepath.Join(stateDir, "state.json")})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() {
		if err := dbStore.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	pool := ww.NewPool(dbStore.SqlDB(), workspacesDir)

	name, err := resolveWorkspaceName([]string{"ws-123"}, pool)
	if err != nil {
		t.Fatalf("failed to resolve name: %v", err)
	}

	if name != "ws-123" {
		t.Fatalf("expected ws-123, got %q", name)
	}
}

func TestValidateWorkspaceAcquirePurpose(t *testing.T) {
	cases := []struct {
		name    string
		purpose string
		wantErr string
	}{
		{name: "empty", purpose: "", wantErr: "purpose is required"},
		{name: "whitespace", purpose: "  ", wantErr: "purpose is required"},
		{name: "multiline", purpose: "first\nsecond", wantErr: "purpose must be a single line"},
		{name: "valid", purpose: "debugging cache", wantErr: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ww.ValidateAcquirePurpose(tc.purpose)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestResolveWorkspaceNameFromCwd(t *testing.T) {
	stateDir := t.TempDir()
	workspacesDir := t.TempDir()
	workspacesDir, _ = filepath.EvalSymlinks(workspacesDir)

	dbPath := filepath.Join(stateDir, "state.db")
	dbStore, err := db.Open(dbPath, db.OpenOptions{LegacyJSONPath: filepath.Join(stateDir, "state.json")})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() {
		if err := dbStore.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	pool := ww.NewPool(dbStore.SqlDB(), workspacesDir)
	repoPath := setupTestRepo(t)

	wsPath, err := pool.Acquire(repoPath, ww.AcquireOptions{Purpose: "workspace test"})
	if err != nil {
		t.Fatalf("failed to acquire workspace: %v", err)
	}

	expected := filepath.Base(wsPath)
	withCwd(t, wsPath, func() {
		name, err := resolveWorkspaceName(nil, pool)
		if err != nil {
			t.Fatalf("failed to resolve name: %v", err)
		}
		if name != expected {
			t.Fatalf("expected %q, got %q", expected, name)
		}
	})
}

func TestResolveWorkspaceNameFromCwd_NotInWorkspace(t *testing.T) {
	stateDir := t.TempDir()
	workspacesDir := t.TempDir()
	workspacesDir, _ = filepath.EvalSymlinks(workspacesDir)

	dbPath := filepath.Join(stateDir, "state.db")
	dbStore, err := db.Open(dbPath, db.OpenOptions{LegacyJSONPath: filepath.Join(stateDir, "state.json")})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() {
		if err := dbStore.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	pool := ww.NewPool(dbStore.SqlDB(), workspacesDir)
	repoPath := setupTestRepo(t)

	withCwd(t, repoPath, func() {
		_, err := resolveWorkspaceName(nil, pool)
		if err == nil {
			t.Fatal("expected error when not in a workspace")
		}
		if !errors.Is(err, ww.ErrRepoPathNotFound) {
			t.Fatalf("expected repo path not found error, got %v", err)
		}
	})
}
