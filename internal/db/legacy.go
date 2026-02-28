package db

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const legacyMigrationPrompt = "Incrementum needs to migrate state from JSON to SQLite. Continue?"

type legacyState struct {
	Repos         map[string]legacyRepoInfo     `json:"repos"`
	Workspaces    map[string]legacyWorkspace    `json:"workspaces"`
	AgentSessions map[string]legacyAgentSession `json:"agent_sessions"`
	Jobs          map[string]legacyJob          `json:"jobs"`
}

type legacyRepoInfo struct {
	SourcePath string `json:"source_path"`
}

type legacyJobStage string

type legacyJobStatus string

type legacyReviewOutcome string

const (
	legacyJobStageImplementing legacyJobStage = "implementing"
	legacyJobStageTesting      legacyJobStage = "testing"
	legacyJobStageReviewing    legacyJobStage = "reviewing"
	legacyJobStageCommitting   legacyJobStage = "committing"
)

const (
	legacyJobStatusActive    legacyJobStatus = "active"
	legacyJobStatusCompleted legacyJobStatus = "completed"
	legacyJobStatusFailed    legacyJobStatus = "failed"
	legacyJobStatusAbandoned legacyJobStatus = "abandoned"
)

const (
	legacyReviewOutcomeAccept         legacyReviewOutcome = "ACCEPT"
	legacyReviewOutcomeRequestChanges legacyReviewOutcome = "REQUEST_CHANGES"
	legacyReviewOutcomeAbandon        legacyReviewOutcome = "ABANDON"
)

type legacyJobReview struct {
	Outcome        legacyReviewOutcome `json:"outcome"`
	Comments       string              `json:"comments,omitempty"`
	AgentSessionID string              `json:"agent_session_id"`
	ReviewedAt     time.Time           `json:"reviewed_at"`
}

type legacyJobCommit struct {
	CommitID       string           `json:"commit_id"`
	DraftMessage   string           `json:"draft_message"`
	TestsPassed    *bool            `json:"tests_passed,omitempty"`
	Review         *legacyJobReview `json:"review,omitempty"`
	AgentSessionID string           `json:"agent_session_id"`
	CreatedAt      time.Time        `json:"created_at"`
}

type legacyJobChange struct {
	ChangeID  string            `json:"change_id"`
	Commits   []legacyJobCommit `json:"commits"`
	CreatedAt time.Time         `json:"created_at"`
}

type legacyJobAgentSession struct {
	Purpose string `json:"purpose"`
	ID      string `json:"id"`
}

type legacyJob struct {
	ID                  string                 `json:"id"`
	Repo                string                 `json:"repo"`
	TodoID              string                 `json:"todo_id"`
	Agent               string                 `json:"agent"`
	ImplementationModel string                 `json:"implementation_model,omitempty"`
	CodeReviewModel     string                 `json:"code_review_model,omitempty"`
	ProjectReviewModel  string                 `json:"project_review_model,omitempty"`
	Stage               legacyJobStage         `json:"stage"`
	Feedback            string                 `json:"feedback,omitempty"`
	AgentSessions       []legacyJobAgentSession `json:"agent_sessions,omitempty"`
	Changes             []legacyJobChange       `json:"changes,omitempty"`
	ProjectReview       *legacyJobReview        `json:"project_review,omitempty"`
	Status              legacyJobStatus         `json:"status"`
	CreatedAt           time.Time               `json:"created_at"`
	StartedAt           time.Time               `json:"started_at"`
	UpdatedAt           time.Time               `json:"updated_at"`
	CompletedAt         time.Time               `json:"completed_at"`
}

type legacyAgentSession struct {
	ID              string    `json:"id"`
	Repo            string    `json:"repo"`
	Status          string    `json:"status"`
	Model           string    `json:"model"`
	CreatedAt       time.Time `json:"created_at"`
	StartedAt       time.Time `json:"started_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	CompletedAt     time.Time `json:"completed_at"`
	ExitCode        *int      `json:"exit_code,omitempty"`
	DurationSeconds int       `json:"duration_seconds,omitempty"`
	TokensUsed      int       `json:"tokens_used,omitempty"`
	Cost            float64   `json:"cost,omitempty"`
}

type legacyWorkspace struct {
	Name          string    `json:"name"`
	Repo          string    `json:"repo"`
	Path          string    `json:"path"`
	Purpose       string    `json:"purpose"`
	Rev           string    `json:"rev"`
	Status        string    `json:"status"`
	AcquiredByPID int       `json:"acquired_by_pid"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	AcquiredAt    time.Time `json:"acquired_at"`
	Provisioned   bool      `json:"provisioned"`
}

func confirmLegacyMigration() (bool, error) {
	fmt.Fprintf(os.Stderr, "%s [Y/n]: ", legacyMigrationPrompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	response = strings.TrimSpace(response)
	if response == "" {
		return true, nil
	}
	switch strings.ToLower(response) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, nil
	}
}

func importLegacyState(db *sql.DB, legacyPath string) error {
	st, err := loadLegacyState(legacyPath)
	if err != nil {
		return err
	}

	if err := insertLegacyState(db, st); err != nil {
		return err
	}

	backupPath := legacyPath + ".bak"
	if err := os.Rename(legacyPath, backupPath); err != nil {
		return fmt.Errorf("legacy migration: rename %q to %q: %w", legacyPath, backupPath, err)
	}

	return nil
}

func loadLegacyState(path string) (*legacyState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("legacy migration: read %q: %w", path, err)
	}
	var st legacyState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("legacy migration: decode %q: %w", path, err)
	}
	ensureLegacyMaps(&st)
	return &st, nil
}

func ensureLegacyMaps(st *legacyState) {
	if st.Repos == nil {
		st.Repos = make(map[string]legacyRepoInfo)
	}
	if st.Workspaces == nil {
		st.Workspaces = make(map[string]legacyWorkspace)
	}
	if st.AgentSessions == nil {
		st.AgentSessions = make(map[string]legacyAgentSession)
	}
	if st.Jobs == nil {
		st.Jobs = make(map[string]legacyJob)
	}
}

func insertLegacyState(db *sql.DB, st *legacyState) error {
	if st == nil {
		return fmt.Errorf("legacy migration: state is nil")
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("legacy migration: begin: %w", err)
	}

	if err := insertLegacyRepos(tx, st.Repos); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := insertLegacyWorkspaces(tx, st.Workspaces); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := insertLegacyAgentSessions(tx, st.AgentSessions); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := insertLegacyJobs(tx, st.Jobs); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("legacy migration: commit: %w", err)
	}
	return nil
}

func insertLegacyRepos(tx *sql.Tx, repos map[string]legacyRepoInfo) error {
	stmt, err := tx.Prepare("INSERT INTO repos (name, source_path) VALUES (?, ?);")
	if err != nil {
		return fmt.Errorf("legacy migration: prepare repos: %w", err)
	}
	defer stmt.Close()

	for name, info := range repos {
		if _, err := stmt.Exec(name, info.SourcePath); err != nil {
			return fmt.Errorf("legacy migration: insert repo %q: %w", name, err)
		}
	}
	return nil
}

func insertLegacyWorkspaces(tx *sql.Tx, workspaces map[string]legacyWorkspace) error {
	stmt, err := tx.Prepare(`INSERT INTO workspaces (
		repo, name, path, purpose, rev, status, acquired_by_pid, provisioned,
		created_at, updated_at, acquired_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`)
	if err != nil {
		return fmt.Errorf("legacy migration: prepare workspaces: %w", err)
	}
	defer stmt.Close()

	for _, ws := range workspaces {
		var acquiredBy any
		if ws.AcquiredByPID != 0 {
			acquiredBy = ws.AcquiredByPID
		}
		acquiredAt := formatLegacyTime(ws.AcquiredAt)
		if ws.AcquiredAt.IsZero() {
			acquiredAt = ""
		}
		if _, err := stmt.Exec(
			ws.Repo,
			ws.Name,
			ws.Path,
			ws.Purpose,
			ws.Rev,
			ws.Status,
			acquiredBy,
			boolToSQLite(ws.Provisioned),
			formatLegacyTime(ws.CreatedAt),
			formatLegacyTime(ws.UpdatedAt),
			acquiredAt,
		); err != nil {
			return fmt.Errorf("legacy migration: insert workspace %q/%q: %w", ws.Repo, ws.Name, err)
		}
	}
	return nil
}

func insertLegacyAgentSessions(tx *sql.Tx, sessions map[string]legacyAgentSession) error {
	stmt, err := tx.Prepare(`INSERT INTO agent_sessions (
		repo, id, status, model, created_at, started_at, updated_at,
		completed_at, exit_code, duration_seconds, tokens_used, cost
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`)
	if err != nil {
		return fmt.Errorf("legacy migration: prepare agent_sessions: %w", err)
	}
	defer stmt.Close()

	for _, session := range sessions {
		var exitCode any
		if session.ExitCode != nil {
			exitCode = *session.ExitCode
		}
		startedAt := formatLegacyTime(session.StartedAt)
		if session.StartedAt.IsZero() {
			startedAt = ""
		}
		completedAt := formatLegacyTime(session.CompletedAt)
		if session.CompletedAt.IsZero() {
			completedAt = ""
		}
		if _, err := stmt.Exec(
			session.Repo,
			session.ID,
			session.Status,
			session.Model,
			formatLegacyTime(session.CreatedAt),
			startedAt,
			formatLegacyTime(session.UpdatedAt),
			completedAt,
			exitCode,
			session.DurationSeconds,
			session.TokensUsed,
			session.Cost,
		); err != nil {
			return fmt.Errorf("legacy migration: insert agent session %q: %w", session.ID, err)
		}
	}
	return nil
}

func insertLegacyJobs(tx *sql.Tx, jobs map[string]legacyJob) error {
	stmt, err := tx.Prepare(`INSERT INTO jobs (
		repo, id, todo_id, agent, implementation_model, code_review_model,
		project_review_model, stage, status, feedback,
		project_review_outcome, project_review_comments,
		project_review_agent_session_id, project_review_reviewed_at,
		created_at, started_at, updated_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`)
	if err != nil {
		return fmt.Errorf("legacy migration: prepare jobs: %w", err)
	}
	defer stmt.Close()

	for _, job := range jobs {
		projectOutcome, projectComments, projectAgentID, projectReviewedAt := legacyReviewFields(job.ProjectReview)
		jobCompletedAt := formatLegacyTime(job.CompletedAt)
		if job.CompletedAt.IsZero() {
			jobCompletedAt = ""
		}
		jobStartedAt := formatLegacyTime(job.StartedAt)
		if job.StartedAt.IsZero() {
			jobStartedAt = ""
		}
		if _, err := stmt.Exec(
			job.Repo,
			job.ID,
			job.TodoID,
			job.Agent,
			job.ImplementationModel,
			job.CodeReviewModel,
			job.ProjectReviewModel,
			string(job.Stage),
			string(job.Status),
			job.Feedback,
			projectOutcome,
			projectComments,
			projectAgentID,
			projectReviewedAt,
			formatLegacyTime(job.CreatedAt),
			jobStartedAt,
			formatLegacyTime(job.UpdatedAt),
			jobCompletedAt,
		); err != nil {
			return fmt.Errorf("legacy migration: insert job %q: %w", job.ID, err)
		}

		if err := insertLegacyJobSessions(tx, job); err != nil {
			return err
		}
		if err := insertLegacyJobChanges(tx, job); err != nil {
			return err
		}
	}

	return nil
}

func insertLegacyJobSessions(tx *sql.Tx, job legacyJob) error {
	stmt, err := tx.Prepare(`INSERT INTO job_agent_sessions (
		repo, job_id, session_id, purpose, position
	) VALUES (?, ?, ?, ?, ?);`)
	if err != nil {
		return fmt.Errorf("legacy migration: prepare job_agent_sessions: %w", err)
	}
	defer stmt.Close()

	for idx, session := range job.AgentSessions {
		if _, err := stmt.Exec(
			job.Repo,
			job.ID,
			session.ID,
			session.Purpose,
			idx,
		); err != nil {
			return fmt.Errorf("legacy migration: insert job agent session %q: %w", session.ID, err)
		}
	}
	return nil
}

func insertLegacyJobChanges(tx *sql.Tx, job legacyJob) error {
	stmtChange, err := tx.Prepare(`INSERT INTO job_changes (
		repo, job_id, change_id, created_at, position
	) VALUES (?, ?, ?, ?, ?);`)
	if err != nil {
		return fmt.Errorf("legacy migration: prepare job_changes: %w", err)
	}
	defer stmtChange.Close()

	stmtCommit, err := tx.Prepare(`INSERT INTO job_commits (
		job_change_id, commit_id, draft_message, tests_passed, agent_session_id,
		review_outcome, review_comments, review_agent_session_id, review_reviewed_at,
		created_at, position
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`)
	if err != nil {
		return fmt.Errorf("legacy migration: prepare job_commits: %w", err)
	}
	defer stmtCommit.Close()

	for changeIndex, change := range job.Changes {
		result, err := stmtChange.Exec(
			job.Repo,
			job.ID,
			change.ChangeID,
			formatLegacyTime(change.CreatedAt),
			changeIndex,
		)
		if err != nil {
			return fmt.Errorf("legacy migration: insert job change %q: %w", change.ChangeID, err)
		}

		changeID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("legacy migration: read job change id: %w", err)
		}

		for commitIndex, commit := range change.Commits {
			reviewOutcome, reviewComments, reviewAgentID, reviewReviewedAt := legacyReviewFields(commit.Review)
			reviewCommentsValue, reviewAgentValue, reviewReviewedValue := reviewComments, reviewAgentID, reviewReviewedAt
			if commit.Review == nil {
				reviewCommentsValue = ""
				reviewAgentValue = ""
				reviewReviewedValue = ""
			}
			if _, err := stmtCommit.Exec(
				changeID,
				commit.CommitID,
				commit.DraftMessage,
				boolPointerToSQLite(commit.TestsPassed),
				commit.AgentSessionID,
				reviewOutcome,
				reviewCommentsValue,
				reviewAgentValue,
				reviewReviewedValue,
				formatLegacyTime(commit.CreatedAt),
				commitIndex,
			); err != nil {
				return fmt.Errorf("legacy migration: insert job commit %q: %w", commit.CommitID, err)
			}
		}
	}

	return nil
}

func legacyReviewFields(review *legacyJobReview) (any, string, string, string) {
	if review == nil {
		return nil, "", "", ""
	}
	return string(review.Outcome), review.Comments, review.AgentSessionID, formatLegacyTime(review.ReviewedAt)
}

func boolPointerToSQLite(value *bool) any {
	if value == nil {
		return nil
	}
	if *value {
		return 1
	}
	return 0
}

func boolToSQLite(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatLegacyTime(value time.Time) string {
	if value.IsZero() {
		return time.Time{}.UTC().Format(time.RFC3339Nano)
	}
	return value.UTC().Format(time.RFC3339Nano)
}
