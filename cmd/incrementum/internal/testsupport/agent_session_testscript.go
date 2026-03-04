package testsupport

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/db"
	"github.com/amonks/incrementum/internal/paths"
	"github.com/rogpeppe/go-internal/testscript"
)

// CmdAgentSession upserts an agent session in the SQLite state database.
//
// Usage: agent-session REPO_PATH SESSION_ID STATUS CREATED_AT UPDATED_AT COMPLETED_AT EXIT_CODE
// Use "-" for COMPLETED_AT or EXIT_CODE to indicate an empty value.
func CmdAgentSession(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("agent-session does not support negation")
	}
	if len(args) != 7 {
		ts.Fatalf("usage: agent-session REPO_PATH SESSION_ID STATUS CREATED_AT UPDATED_AT COMPLETED_AT EXIT_CODE")
	}

	repoPath := args[0]
	sessionID := args[1]
	status := args[2]
	createdAt, err := parseAgentSessionTime(args[3])
	if err != nil {
		ts.Fatalf("parse created_at: %v", err)
	}
	updatedAt, err := parseAgentSessionTime(args[4])
	if err != nil {
		ts.Fatalf("parse updated_at: %v", err)
	}
	completedAt, err := parseAgentSessionOptionalTime(args[5])
	if err != nil {
		ts.Fatalf("parse completed_at: %v", err)
	}
	exitCode, err := parseAgentSessionExitCode(args[6])
	if err != nil {
		ts.Fatalf("parse exit_code: %v", err)
	}

	stateDirValue := ts.Getenv("INCREMENTUM_STATE_DIR")
	if stateDirValue == "" {
		stateDirValue = os.Getenv("INCREMENTUM_STATE_DIR")
	}
	stateDir, err := paths.ResolveWithDefault(stateDirValue, paths.DefaultStateDir)
	if err != nil {
		ts.Fatalf("resolve state dir: %v", err)
	}

	dbPath := filepath.Join(stateDir, "state.db")
	store, err := db.Open(dbPath, db.OpenOptions{})
	if err != nil {
		ts.Fatalf("open db: %v", err)
	}
	defer store.Close()

	sqlDB := store.SqlDB()
	repoName, err := db.GetOrCreateRepoName(sqlDB, repoPath)
	if err != nil {
		ts.Fatalf("get repo name: %v", err)
	}

	durationSeconds := 0
	if !completedAt.IsZero() {
		durationSeconds = int(completedAt.Sub(createdAt).Seconds())
		if durationSeconds < 0 {
			durationSeconds = 0
		}
	}

	if err := upsertAgentSession(sqlDB, repoName, sessionID, status, createdAt, updatedAt, completedAt, exitCode, durationSeconds); err != nil {
		ts.Fatalf("upsert agent session: %v", err)
	}
}

func parseAgentSessionTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" || value == "-" {
		return time.Time{}, fmt.Errorf("time value is required")
	}
	return time.Parse(time.RFC3339Nano, value)
}

func parseAgentSessionOptionalTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" || value == "-" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func parseAgentSessionExitCode(value string) (sql.NullInt64, error) {
	if strings.TrimSpace(value) == "" || value == "-" {
		return sql.NullInt64{Valid: false}, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return sql.NullInt64{}, err
	}
	return sql.NullInt64{Int64: int64(parsed), Valid: true}, nil
}

func upsertAgentSession(dbHandle *sql.DB, repoName, sessionID, status string, createdAt, updatedAt, completedAt time.Time, exitCode sql.NullInt64, durationSeconds int) error {
	_, err := dbHandle.Exec(`INSERT INTO agent_sessions (
		repo, id, status, model, created_at, started_at, updated_at,
		completed_at, exit_code, duration_seconds, tokens_used, cost
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(repo, id) DO UPDATE SET
		status = excluded.status,
		model = excluded.model,
		created_at = excluded.created_at,
		started_at = excluded.started_at,
		updated_at = excluded.updated_at,
		completed_at = excluded.completed_at,
		exit_code = excluded.exit_code,
		duration_seconds = excluded.duration_seconds,
		tokens_used = excluded.tokens_used,
		cost = excluded.cost;`,
		repoName,
		sessionID,
		status,
		"test",
		createdAt.UTC().Format(time.RFC3339Nano),
		createdAt.UTC().Format(time.RFC3339Nano),
		updatedAt.UTC().Format(time.RFC3339Nano),
		formatOptionalAgentSessionTime(completedAt),
		exitCode,
		durationSeconds,
		0,
		0,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func formatOptionalAgentSessionTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
