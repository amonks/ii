package agent_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"monks.co/ii/agent"
	"monks.co/ii/internal/db"
)

func openAgentTestDB(t *testing.T, stateDir string) *sql.DB {
	t.Helper()

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	store, err := db.Open(filepath.Join(stateDir, "state.db"), db.OpenOptions{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	return store.SqlDB()
}

func insertAgentSession(
	sqlDB *sql.DB,
	repoName string,
	id string,
	status agent.SessionStatus,
	model string,
	createdAt time.Time,
	startedAt time.Time,
	updatedAt time.Time,
	completedAt time.Time,
	exitCode *int,
	durationSeconds int,
	tokensUsed int,
	cost float64,
) error {
	var exitValue any
	if exitCode != nil {
		exitValue = *exitCode
	}

	startValue := formatSessionTime(startedAt)
	if startedAt.IsZero() {
		startValue = ""
	}
	completedValue := formatSessionTime(completedAt)
	if completedAt.IsZero() {
		completedValue = ""
	}

	_, err := sqlDB.Exec(`INSERT INTO agent_sessions (
		repo, id, status, model, created_at, started_at, updated_at,
		completed_at, exit_code, duration_seconds, tokens_used, cost
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		repoName,
		id,
		string(status),
		model,
		formatSessionTime(createdAt),
		startValue,
		formatSessionTime(updatedAt),
		completedValue,
		exitValue,
		durationSeconds,
		tokensUsed,
		cost,
	)
	return err
}

func formatSessionTime(value time.Time) string {
	if value.IsZero() {
		return time.Time{}.UTC().Format(time.RFC3339Nano)
	}
	return value.UTC().Format(time.RFC3339Nano)
}
