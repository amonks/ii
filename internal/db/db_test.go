package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/amonks/incrementum/internal/testsupport"
)

func TestOpenMigratesAndPragmas(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(dbPath, OpenOptions{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	assertTableExists(t, store.sql, "repos")
	assertTableExists(t, store.sql, "workspaces")
	assertTableExists(t, store.sql, "agent_sessions")
	assertTableExists(t, store.sql, "jobs")
	assertTableExists(t, store.sql, "job_agent_sessions")
	assertTableExists(t, store.sql, "job_changes")
	assertTableExists(t, store.sql, "job_commits")

	assertPragma(t, store.sql, "journal_mode", "wal")
	assertPragma(t, store.sql, "busy_timeout", "5000")
	assertPragma(t, store.sql, "foreign_keys", "1")
	assertPragma(t, store.sql, "synchronous", "1")
	assertPragma(t, store.sql, "cache_size", "-2000")
}

func TestOpenIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(dbPath, OpenOptions{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	store, err = Open(dbPath, OpenOptions{})
	if err != nil {
		t.Fatalf("open db second time: %v", err)
	}
	defer store.Close()

	assertSchemaVersion(t, store.sql, 1)
}

func TestOpenCreatesParentDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "state.db")
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("expected parent dir to not exist")
	}

	store, err := Open(path, OpenOptions{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("expected parent dir to exist: %v", err)
	}
}

func assertTableExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	row := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?;", name)
	var found string
	if err := row.Scan(&found); err != nil {
		t.Fatalf("expected table %s: %v", name, err)
	}
}

func assertPragma(t *testing.T, db *sql.DB, name string, expected string) {
	t.Helper()
	row := db.QueryRow("PRAGMA " + name + ";")
	var raw any
	if err := row.Scan(&raw); err != nil {
		t.Fatalf("read pragma %s: %v", name, err)
	}
	value := fmt.Sprint(raw)
	if value != expected {
		t.Fatalf("pragma %s = %q, expected %q", name, value, expected)
	}
}

func assertSchemaVersion(t *testing.T, db *sql.DB, expected int) {
	t.Helper()
	row := db.QueryRow("SELECT version FROM schema_version LIMIT 1;")
	var version int
	if err := row.Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != expected {
		t.Fatalf("schema version = %d, expected %d", version, expected)
	}
}

func TestBuildIIRespectsOutputDir(t *testing.T) {
	testsupport.BuildII(t)
}
