package db

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
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

func TestOpenMigratesLegacyJSON(t *testing.T) {
	tmpDir := t.TempDir()
	legacyPath := filepath.Join(tmpDir, "state.json")

	repoPath := filepath.Join(tmpDir, "repo")
	legacyState := statestore.State{
		Repos: map[string]statestore.RepoInfo{
			"repo": {SourcePath: repoPath},
		},
		Workspaces: map[string]statestore.WorkspaceInfo{
			"repo/ws-001": {
				Name:          "ws-001",
				Repo:          "repo",
				Path:          filepath.Join(tmpDir, "ws-001"),
				Purpose:       "testing",
				Rev:           "@",
				Status:        statestore.WorkspaceStatusAcquired,
				AcquiredByPID: 4242,
				Provisioned:   true,
				CreatedAt:     time.Date(2026, 1, 1, 1, 2, 3, 0, time.UTC),
				UpdatedAt:     time.Date(2026, 1, 2, 1, 2, 3, 0, time.UTC),
				AcquiredAt:    time.Date(2026, 1, 3, 1, 2, 3, 0, time.UTC),
			},
		},
		AgentSessions: map[string]statestore.AgentSession{
			"repo/abc123": {
				ID:              "abc123",
				Repo:            "repo",
				Status:          statestore.AgentSessionCompleted,
				Model:           "test",
				CreatedAt:       time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC),
				StartedAt:       time.Time{},
				UpdatedAt:       time.Date(2026, 1, 1, 2, 15, 0, 0, time.UTC),
				CompletedAt:     time.Date(2026, 1, 1, 2, 20, 0, 0, time.UTC),
				ExitCode:        intPointer(0),
				DurationSeconds: 120,
				TokensUsed:      42,
				Cost:            0.5,
			},
		},
		Jobs: map[string]statestore.Job{
			"repo/job-1": {
				ID:                  "job-1",
				Repo:                "repo",
				TodoID:              "todo-1",
				Agent:               "agent",
				ImplementationModel: "impl",
				CodeReviewModel:     "review",
				ProjectReviewModel:  "project",
				Stage:               statestore.JobStageReviewing,
				Feedback:            "feedback",
				AgentSessions: []statestore.JobAgentSession{
					{Purpose: "implementation", ID: "abc123"},
				},
				Changes: []statestore.JobChange{
					{
						ChangeID:  "change-1",
						CreatedAt: time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC),
						Commits: []statestore.JobCommit{
							{
								CommitID:       "commit-1",
								DraftMessage:   "draft",
								TestsPassed:    boolPointer(true),
								AgentSessionID: "abc123",
								Review: &statestore.JobReview{
									Outcome:        statestore.ReviewOutcomeAccept,
									Comments:       "looks good",
									AgentSessionID: "abc123",
									ReviewedAt:     time.Date(2026, 1, 1, 4, 0, 0, 0, time.UTC),
								},
								CreatedAt: time.Date(2026, 1, 1, 3, 30, 0, 0, time.UTC),
							},
						},
					},
				},
				ProjectReview: &statestore.JobReview{
					Outcome:        statestore.ReviewOutcomeRequestChanges,
					Comments:       "needs more",
					AgentSessionID: "abc123",
					ReviewedAt:     time.Date(2026, 1, 2, 4, 0, 0, 0, time.UTC),
				},
				Status:      statestore.JobStatusActive,
				CreatedAt:   time.Date(2026, 1, 1, 2, 30, 0, 0, time.UTC),
				StartedAt:   time.Time{},
				UpdatedAt:   time.Date(2026, 1, 1, 2, 40, 0, 0, time.UTC),
				CompletedAt: time.Time{},
			},
		},
	}

	if err := writeLegacyState(legacyPath, legacyState); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "state.db")
	store, err := Open(dbPath, OpenOptions{LegacyJSONPath: legacyPath, SkipConfirm: true})
	if err != nil {
		t.Fatalf("open db with legacy: %v", err)
	}
	defer store.Close()

	assertLegacyMigration(t, store.sql, legacyState)

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy state.json removed, got %v", err)
	}
	if _, err := os.Stat(legacyPath + ".bak"); err != nil {
		t.Fatalf("expected legacy state backup: %v", err)
	}
}

func TestOpenLegacyMigrationDeclined(t *testing.T) {
	tmpDir := t.TempDir()
	legacyPath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(legacyPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write legacy json: %v", err)
	}

	stdin := os.Stdin
	stderr := os.Stderr
	stdout := os.Stdout
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stdin = stdinReader
	os.Stderr = stderrWriter
	os.Stdout = stderrWriter
	defer func() {
		os.Stdin = stdin
		os.Stderr = stderr
		os.Stdout = stdout
	}()

	if _, err := stdinWriter.WriteString("n\n"); err != nil {
		t.Fatalf("write prompt response: %v", err)
	}
	if err := stdinWriter.Close(); err != nil {
		t.Fatalf("close prompt writer: %v", err)
	}

	_, err = Open(filepath.Join(tmpDir, "state.db"), OpenOptions{LegacyJSONPath: legacyPath})
	if !errors.Is(err, ErrLegacyMigrationDeclined) {
		t.Fatalf("expected legacy migration declined error, got %v", err)
	}

	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	prompt := readAll(t, stderrReader)
	if !strings.Contains(prompt, legacyMigrationPrompt) {
		t.Fatalf("expected prompt to mention migration, got %q", prompt)
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

func writeLegacyState(path string, st statestore.State) error {
	payload, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func assertLegacyMigration(t *testing.T, db *sql.DB, st statestore.State) {
	assertLegacyRepo(t, db, st)
	assertLegacyWorkspace(t, db, st)
	assertLegacyAgentSession(t, db, st)
	assertLegacyJob(t, db, st)

	assertLegacyZeroDefaults(t, db, st)
}

func assertLegacyZeroDefaults(t *testing.T, db *sql.DB, st statestore.State) {
	row := db.QueryRow("SELECT acquired_at FROM workspaces WHERE repo = ? AND name = ?;", "repo", "ws-001")
	var acquiredAt string
	if err := row.Scan(&acquiredAt); err != nil {
		t.Fatalf("scan acquired_at: %v", err)
	}
	assertLegacyTime(t, acquiredAt, st.Workspaces["repo/ws-001"].AcquiredAt)

	row = db.QueryRow("SELECT started_at FROM agent_sessions WHERE repo = ? AND id = ?;", "repo", "abc123")
	var startedAt string
	if err := row.Scan(&startedAt); err != nil {
		t.Fatalf("scan started_at: %v", err)
	}
	assertLegacyTime(t, startedAt, st.AgentSessions["repo/abc123"].StartedAt)

	row = db.QueryRow("SELECT completed_at FROM jobs WHERE repo = ? AND id = ?;", "repo", "job-1")
	var completedAt string
	if err := row.Scan(&completedAt); err != nil {
		t.Fatalf("scan completed_at: %v", err)
	}
	if completedAt != "" {
		t.Fatalf("expected empty completed_at, got %q", completedAt)
	}
}

func assertLegacyRepo(t *testing.T, db *sql.DB, st statestore.State) {
	row := db.QueryRow("SELECT name, source_path FROM repos WHERE name = ?;", "repo")
	var name string
	var source string
	if err := row.Scan(&name, &source); err != nil {
		t.Fatalf("scan repo: %v", err)
	}
	if name != "repo" {
		t.Fatalf("repo name = %q", name)
	}
	if source != st.Repos["repo"].SourcePath {
		t.Fatalf("repo source = %q", source)
	}
}

func assertLegacyWorkspace(t *testing.T, db *sql.DB, st statestore.State) {
	row := db.QueryRow(`SELECT name, path, purpose, rev, status, acquired_by_pid, provisioned,
		created_at, updated_at, acquired_at FROM workspaces WHERE repo = ? AND name = ?;`, "repo", "ws-001")
	var name, path, purpose, rev, status string
	var acquiredBy sql.NullInt64
	var provisioned int
	var createdAt, updatedAt, acquiredAt string
	if err := row.Scan(&name, &path, &purpose, &rev, &status, &acquiredBy, &provisioned, &createdAt, &updatedAt, &acquiredAt); err != nil {
		t.Fatalf("scan workspace: %v", err)
	}
	ws := st.Workspaces["repo/ws-001"]
	if name != ws.Name || path != ws.Path || purpose != ws.Purpose || rev != ws.Rev || status != string(ws.Status) {
		t.Fatalf("workspace mismatch: %+v", ws)
	}
	if !acquiredBy.Valid || int(acquiredBy.Int64) != ws.AcquiredByPID {
		t.Fatalf("workspace acquired pid mismatch: %v", acquiredBy)
	}
	if provisioned != 1 {
		t.Fatalf("workspace provisioned mismatch: %d", provisioned)
	}
	assertLegacyTime(t, createdAt, ws.CreatedAt)
	assertLegacyTime(t, updatedAt, ws.UpdatedAt)
	assertLegacyTime(t, acquiredAt, ws.AcquiredAt)
}

func assertLegacyAgentSession(t *testing.T, db *sql.DB, st statestore.State) {
	row := db.QueryRow(`SELECT status, model, created_at, started_at, updated_at, completed_at,
		exit_code, duration_seconds, tokens_used, cost FROM agent_sessions WHERE repo = ? AND id = ?;`, "repo", "abc123")
	var status, model, createdAt, startedAt, updatedAt, completedAt string
	var exitCode sql.NullInt64
	var duration, tokens int
	var cost float64
	if err := row.Scan(&status, &model, &createdAt, &startedAt, &updatedAt, &completedAt, &exitCode, &duration, &tokens, &cost); err != nil {
		t.Fatalf("scan agent session: %v", err)
	}
	session := st.AgentSessions["repo/abc123"]
	if status != string(session.Status) || model != session.Model {
		t.Fatalf("agent session mismatch: %+v", session)
	}
	if !exitCode.Valid || int(exitCode.Int64) != *session.ExitCode {
		t.Fatalf("exit code mismatch: %v", exitCode)
	}
	if duration != session.DurationSeconds || tokens != session.TokensUsed {
		t.Fatalf("duration/tokens mismatch")
	}
	if cost != session.Cost {
		t.Fatalf("cost mismatch")
	}
	assertLegacyTime(t, createdAt, session.CreatedAt)
	assertLegacyTime(t, startedAt, session.StartedAt)
	assertLegacyTime(t, updatedAt, session.UpdatedAt)
	assertLegacyTime(t, completedAt, session.CompletedAt)
}

func assertLegacyJob(t *testing.T, db *sql.DB, st statestore.State) {
	row := db.QueryRow(`SELECT todo_id, agent, implementation_model, code_review_model,
		project_review_model, stage, status, feedback,
		project_review_outcome, project_review_comments, project_review_agent_session_id,
		project_review_reviewed_at, created_at, started_at, updated_at, completed_at
		FROM jobs WHERE repo = ? AND id = ?;`, "repo", "job-1")
	var todoID, agent, impl, reviewModel, projectModel, stage, status, feedback string
	var projectOutcome sql.NullString
	var projectComments, projectAgentID, projectReviewedAt string
	var createdAt, startedAt, updatedAt, completedAt string
	if err := row.Scan(&todoID, &agent, &impl, &reviewModel, &projectModel, &stage, &status, &feedback, &projectOutcome, &projectComments, &projectAgentID, &projectReviewedAt, &createdAt, &startedAt, &updatedAt, &completedAt); err != nil {
		t.Fatalf("scan job: %v", err)
	}
	job := st.Jobs["repo/job-1"]
	if todoID != job.TodoID || agent != job.Agent || impl != job.ImplementationModel || reviewModel != job.CodeReviewModel {
		t.Fatalf("job metadata mismatch")
	}
	if projectModel != job.ProjectReviewModel || stage != string(job.Stage) || status != string(job.Status) {
		t.Fatalf("job status mismatch")
	}
	if feedback != job.Feedback {
		t.Fatalf("job feedback mismatch")
	}
	if !projectOutcome.Valid || projectOutcome.String != string(job.ProjectReview.Outcome) {
		t.Fatalf("project review outcome mismatch")
	}
	if projectComments != job.ProjectReview.Comments || projectAgentID != job.ProjectReview.AgentSessionID {
		t.Fatalf("project review mismatch")
	}
	assertLegacyTime(t, projectReviewedAt, job.ProjectReview.ReviewedAt)
	assertLegacyTime(t, createdAt, job.CreatedAt)
	assertLegacyTime(t, startedAt, job.StartedAt)
	assertLegacyTime(t, updatedAt, job.UpdatedAt)
	if completedAt != "" {
		t.Fatalf("expected empty completed_at, got %q", completedAt)
	}

	assertLegacyJobSessions(t, db, job)
	assertLegacyJobChanges(t, db, job)
}

func assertLegacyJobSessions(t *testing.T, db *sql.DB, job statestore.Job) {
	rows, err := db.Query(`SELECT session_id, purpose, position FROM job_agent_sessions WHERE repo = ? AND job_id = ? ORDER BY position;`, job.Repo, job.ID)
	if err != nil {
		t.Fatalf("query job sessions: %v", err)
	}
	defer rows.Close()

	var sessions []statestore.JobAgentSession
	for rows.Next() {
		var sessionID, purpose string
		var position int
		if err := rows.Scan(&sessionID, &purpose, &position); err != nil {
			t.Fatalf("scan job session: %v", err)
		}
		sessions = append(sessions, statestore.JobAgentSession{ID: sessionID, Purpose: purpose})
		if position != len(sessions)-1 {
			t.Fatalf("job session position mismatch")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate job sessions: %v", err)
	}
	if len(sessions) != len(job.AgentSessions) {
		t.Fatalf("job sessions count mismatch")
	}
}

func assertLegacyJobChanges(t *testing.T, db *sql.DB, job statestore.Job) {
	rows, err := db.Query(`SELECT id, change_id, created_at, position FROM job_changes WHERE repo = ? AND job_id = ? ORDER BY position;`, job.Repo, job.ID)
	if err != nil {
		t.Fatalf("query job changes: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var changeID string
		var createdAt string
		var position int
		var id int64
		if err := rows.Scan(&id, &changeID, &createdAt, &position); err != nil {
			t.Fatalf("scan job change: %v", err)
		}
		if position >= len(job.Changes) {
			t.Fatalf("job change position out of range")
		}
		change := job.Changes[position]
		if change.ChangeID != changeID {
			t.Fatalf("job change id mismatch")
		}
		assertLegacyTime(t, createdAt, change.CreatedAt)
		assertLegacyJobCommits(t, db, id, change)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate job changes: %v", err)
	}
}

func assertLegacyJobCommits(t *testing.T, db *sql.DB, changeID int64, change statestore.JobChange) {
	rows, err := db.Query(`SELECT commit_id, draft_message, tests_passed, agent_session_id,
		review_outcome, review_comments, review_agent_session_id, review_reviewed_at,
		created_at, position FROM job_commits WHERE job_change_id = ? ORDER BY position;`, changeID)
	if err != nil {
		t.Fatalf("query job commits: %v", err)
	}
	defer rows.Close()

	var commits []statestore.JobCommit
	for rows.Next() {
		var commitID, draftMessage, agentSessionID string
		var testsPassed sql.NullInt64
		var reviewOutcome sql.NullString
		var reviewComments, reviewAgentID, reviewReviewedAt string
		var createdAt string
		var position int
		if err := rows.Scan(&commitID, &draftMessage, &testsPassed, &agentSessionID, &reviewOutcome, &reviewComments, &reviewAgentID, &reviewReviewedAt, &createdAt, &position); err != nil {
			t.Fatalf("scan job commit: %v", err)
		}
		if testsPassed.Valid {
			value := testsPassed.Int64 == 1
			commits = append(commits, statestore.JobCommit{TestsPassed: &value})
		} else {
			commits = append(commits, statestore.JobCommit{})
		}
		if position != len(commits)-1 {
			t.Fatalf("job commit position mismatch")
		}
		commit := change.Commits[position]
		if commit.CommitID != commitID || commit.DraftMessage != draftMessage || commit.AgentSessionID != agentSessionID {
			t.Fatalf("job commit mismatch")
		}
		if commit.TestsPassed != nil {
			if !testsPassed.Valid || (testsPassed.Int64 == 1) != *commit.TestsPassed {
				t.Fatalf("job commit testsPassed mismatch")
			}
		} else if testsPassed.Valid {
			t.Fatalf("unexpected testsPassed")
		}
		if commit.Review != nil {
			if !reviewOutcome.Valid || reviewOutcome.String != string(commit.Review.Outcome) {
				t.Fatalf("job commit review outcome mismatch")
			}
			if reviewComments != commit.Review.Comments || reviewAgentID != commit.Review.AgentSessionID {
				t.Fatalf("job commit review mismatch")
			}
			assertLegacyTime(t, reviewReviewedAt, commit.Review.ReviewedAt)
		} else {
			if reviewOutcome.Valid {
				t.Fatalf("unexpected review outcome")
			}
			if reviewComments != "" || reviewAgentID != "" || reviewReviewedAt != "" {
				t.Fatalf("unexpected review fields")
			}
		}
		assertLegacyTime(t, createdAt, commit.CreatedAt)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate job commits: %v", err)
	}
}

func assertLegacyTime(t *testing.T, value string, expected time.Time) {
	if expected.IsZero() {
		if value != "" {
			t.Fatalf("time mismatch: %q", value)
		}
		return
	}
	if value != formatLegacyTime(expected) {
		t.Fatalf("time mismatch: %q", value)
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func intPointer(value int) *int {
	return &value
}

func readAll(t *testing.T, reader *os.File) string {
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read buffer: %v", err)
	}
	return string(bytes.TrimSpace(data))
}
