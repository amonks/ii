package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestSanitizeRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/Users/test/my-project", "users-test-my-project"},
		{"/Users/test/My Project", "users-test-my-project"},
		{"/home/user/some/deep/path", "home-user-some-deep-path"},
	}

	for _, tt := range tests {
		result := SanitizeRepoName(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeRepoName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetOrCreateRepoName(t *testing.T) {
	db := openTestDB(t)

	name1, err := GetOrCreateRepoName(db, "/Users/test/my-project")
	if err != nil {
		t.Fatalf("failed to get repo name: %v", err)
	}

	name2, err := GetOrCreateRepoName(db, "/Users/test/my-project")
	if err != nil {
		t.Fatalf("failed to get repo name: %v", err)
	}

	if name1 != name2 {
		t.Errorf("expected same name, got %q and %q", name1, name2)
	}

	name3, err := GetOrCreateRepoName(db, "/Users/test/my/project")
	if err != nil {
		t.Fatalf("failed to get repo name: %v", err)
	}

	if name3 == name1 {
		t.Error("collision not handled - different paths got same name")
	}
}

func TestRepoPathForWorkspace(t *testing.T) {
	db := openTestDB(t)

	repoName, err := GetOrCreateRepoName(db, "/Users/test/my-project")
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	wsPath := filepath.Join("/tmp/workspaces", repoName, "ws-001")
	_, err = db.Exec(`INSERT INTO workspaces (repo, name, path, purpose, rev, status, acquired_by_pid,
		provisioned, created_at, updated_at, acquired_at)
		VALUES (?, ?, ?, '', '', ?, NULL, 0, ?, ?, '');`, repoName, "ws-001", wsPath, "available", time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	resolved, found, err := RepoPathForWorkspace(db, wsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected workspace to be found")
	}
	if resolved != "/Users/test/my-project" {
		t.Fatalf("expected repo path, got %q", resolved)
	}

	_, found, err = RepoPathForWorkspace(db, filepath.Join("/tmp/workspaces", repoName, "ws-999"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected workspace to be missing")
	}
}


func TestRepoPathForWorkspaceMissingRepo(t *testing.T) {
	db := openTestDB(t)

	_, err := db.Exec("INSERT INTO repos (name, source_path) VALUES (?, '');", "missing")
	if err != nil {
		t.Fatalf("insert repo: %v", err)
	}

	wsPath := filepath.Join("/tmp/workspaces", "missing", "ws-001")
	_, err = db.Exec(`INSERT INTO workspaces (repo, name, path, purpose, rev, status, acquired_by_pid,
		provisioned, created_at, updated_at, acquired_at)
		VALUES ('missing', 'ws-001', ?, '', '', ?, NULL, 0, ?, ?, '');`, wsPath, "available", time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	_, found, err := RepoPathForWorkspace(db, wsPath)
	if !found {
		t.Fatal("expected workspace to be found")
	}
	if err == nil {
		t.Fatal("expected error for missing repo path")
	}
}

func TestRepoNameForPath(t *testing.T) {
	db := openTestDB(t)

	repoName, err := GetOrCreateRepoName(db, "/Users/test/my-project")
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	name, err := RepoNameForPath(db, "/Users/test/my-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != repoName {
		t.Fatalf("expected repo name %q, got %q", repoName, name)
	}

	name, err = RepoNameForPath(db, "/Users/test/missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "" {
		t.Fatalf("expected empty name, got %q", name)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	store, err := Open(dbPath, OpenOptions{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	return store.SqlDB()
}
