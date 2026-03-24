package job

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"monks.co/ii/internal/db"
	"monks.co/ii/internal/ids"
	"monks.co/ii/internal/paths"
	internalstrings "monks.co/ii/internal/strings"
)

// StaleJobTimeout is the duration after which an active job is considered stale
// and should be marked as failed. Jobs that haven't been updated within this
// duration are assumed to be orphaned (e.g., the process crashed or was killed).
const StaleJobTimeout = 10 * time.Minute

// OpenOptions configures a job manager.
type OpenOptions struct {
	// StateDir is the directory where job state is stored.
	StateDir string

	// DB is an existing SQLite database connection to use.
	// If set, StateDir is ignored for persistence.
	DB *sql.DB
}

// Manager provides access to job state for a repo.
type Manager struct {
	repoPath string
	sqlDB    *sql.DB
	closeDB  func() error
}

// Open opens a job manager for the given repo.
func Open(repoPath string, opts OpenOptions) (*Manager, error) {
	sqlDB, closeFn, err := resolveDB(opts)
	if err != nil {
		return nil, err
	}

	return &Manager{
		repoPath: repoPath,
		sqlDB:    sqlDB,
		closeDB:  closeFn,
	}, nil
}

// Close closes any owned database connection.
func (m *Manager) Close() error {
	if m == nil || m.closeDB == nil {
		return nil
	}
	return m.closeDB()
}

// CreateOptions configures new job creation.
type CreateOptions struct {
	Agent               string
	ImplementationModel string
	CodeReviewModel     string
	ProjectReviewModel  string
}

// Create stores a new job with active status and implementing stage.
func (m *Manager) Create(todoID string, startedAt time.Time, opts CreateOptions) (Job, error) {
	if internalstrings.IsBlank(todoID) {
		return Job{}, fmt.Errorf("todo id is required")
	}

	repoName, err := db.GetOrCreateRepoName(m.sqlDB, m.repoPath)
	if err != nil {
		return Job{}, fmt.Errorf("get repo name: %w", err)
	}

	jobID := GenerateID(todoID, startedAt)
	created := Job{
		ID:                  jobID,
		Repo:                repoName,
		TodoID:              todoID,
		Agent:               internalstrings.TrimSpace(opts.Agent),
		ImplementationModel: internalstrings.TrimSpace(opts.ImplementationModel),
		CodeReviewModel:     internalstrings.TrimSpace(opts.CodeReviewModel),
		ProjectReviewModel:  internalstrings.TrimSpace(opts.ProjectReviewModel),
		Stage:               StageImplementing,
		Status:              StatusActive,
		CreatedAt:           startedAt,
		StartedAt:           startedAt,
		UpdatedAt:           startedAt,
	}

	projectOutcome, projectComments, projectAgentID, projectReviewedAt := reviewFields(created.ProjectReview)
	if _, err := m.sqlDB.Exec(`INSERT INTO jobs (
		repo, id, todo_id, agent, implementation_model, code_review_model,
		project_review_model, stage, status, feedback,
		project_review_outcome, project_review_comments,
		project_review_agent_session_id, project_review_reviewed_at,
		created_at, started_at, updated_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		created.Repo,
		created.ID,
		created.TodoID,
		created.Agent,
		created.ImplementationModel,
		created.CodeReviewModel,
		created.ProjectReviewModel,
		string(created.Stage),
		string(created.Status),
		created.Feedback,
		projectOutcome,
		projectComments,
		projectAgentID,
		projectReviewedAt,
		formatJobTime(created.CreatedAt),
		formatOptionalJobTime(created.StartedAt),
		formatJobTime(created.UpdatedAt),
		formatOptionalJobTime(created.CompletedAt),
	); err != nil {
		return Job{}, err
	}

	return created, nil
}

// UpdateOptions configures job updates.
// Nil fields mean "do not update".
type UpdateOptions struct {
	Stage              *Stage
	Status             *Status
	Feedback           *string
	AppendAgentSession *AgentSession
}

// Update updates an existing job by id or prefix.
func (m *Manager) Update(jobID string, opts UpdateOptions, updatedAt time.Time) (Job, error) {
	if internalstrings.IsBlank(jobID) {
		return Job{}, ErrJobNotFound
	}

	if opts.Stage != nil {
		normalized := normalizeStage(*opts.Stage)
		opts.Stage = &normalized
		if !opts.Stage.IsValid() {
			return Job{}, formatInvalidStageError(*opts.Stage)
		}
	}
	if opts.Status != nil {
		normalized := normalizeStatus(*opts.Status)
		opts.Status = &normalized
		if !opts.Status.IsValid() {
			return Job{}, formatInvalidStatusError(*opts.Status)
		}
	}

	found, err := m.Find(jobID)
	if err != nil {
		return Job{}, err
	}

	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	updated := found
	if opts.Stage != nil {
		updated.Stage = *opts.Stage
	}
	if opts.Status != nil {
		updated.Status = *opts.Status
		if updated.Status != StatusActive {
			updated.CompletedAt = updatedAt
		}
	}
	if opts.Feedback != nil {
		updated.Feedback = *opts.Feedback
	}
	if opts.AppendAgentSession != nil {
		if !agentSessionExists(found.AgentSessions, opts.AppendAgentSession.ID) {
			updated.AgentSessions = append(updated.AgentSessions, JobAgentSession{
				Purpose: opts.AppendAgentSession.Purpose,
				ID:      opts.AppendAgentSession.ID,
			})
		}
	}
	updated.UpdatedAt = updatedAt

	tx, err := m.sqlDB.Begin()
	if err != nil {
		return Job{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`UPDATE jobs SET stage = ?, status = ?, feedback = ?, updated_at = ?, completed_at = ?
		WHERE repo = ? AND id = ?;`,
		string(updated.Stage),
		string(updated.Status),
		updated.Feedback,
		formatJobTime(updated.UpdatedAt),
		formatOptionalJobTime(updated.CompletedAt),
		updated.Repo,
		updated.ID,
	); err != nil {
		return Job{}, err
	}

	if opts.AppendAgentSession != nil {
		if err = ensureAgentSession(tx, updated.Repo, opts.AppendAgentSession.ID, updatedAt); err != nil {
			return Job{}, err
		}
		if !agentSessionExists(found.AgentSessions, opts.AppendAgentSession.ID) {
			position := len(found.AgentSessions)
			if _, err = tx.Exec(`INSERT INTO job_agent_sessions (
				repo, job_id, session_id, purpose, position
			) VALUES (?, ?, ?, ?, ?);`,
				updated.Repo,
				updated.ID,
				opts.AppendAgentSession.ID,
				opts.AppendAgentSession.Purpose,
				position,
			); err != nil {
				return Job{}, err
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return Job{}, err
	}

	return updated, nil
}

// JobCommitUpdate describes in-place updates to the current commit.
// Nil fields mean "do not update".
type JobCommitUpdate struct {
	TestsPassed *bool
	Review      *JobReview
}

// AppendChange appends a change to the job.
func (m *Manager) AppendChange(jobID string, change JobChange, now time.Time) (Job, error) {
	found, err := m.Find(jobID)
	if err != nil {
		return Job{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	if change.CreatedAt.IsZero() {
		change.CreatedAt = now
	}
	if change.Commits == nil {
		change.Commits = make([]JobCommit, 0)
	}

	tx, err := m.sqlDB.Begin()
	if err != nil {
		return Job{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	position := len(found.Changes)
	if _, err = tx.Exec(`INSERT INTO job_changes (
		repo, job_id, change_id, created_at, position
	) VALUES (?, ?, ?, ?, ?);`,
		found.Repo,
		found.ID,
		change.ChangeID,
		formatJobTime(change.CreatedAt),
		position,
	); err != nil {
		return Job{}, err
	}

	if _, err = tx.Exec(`UPDATE jobs SET updated_at = ? WHERE repo = ? AND id = ?;`,
		formatJobTime(now),
		found.Repo,
		found.ID,
	); err != nil {
		return Job{}, err
	}

	if err = tx.Commit(); err != nil {
		return Job{}, err
	}

	return m.loadJobByID(found.Repo, found.ID)
}

// AppendCommitToCurrentChange appends a commit to the job's current change.
// Returns ErrNoCurrentChange if there are no changes, or if the last change is complete.
func (m *Manager) AppendCommitToCurrentChange(jobID string, commit JobCommit, now time.Time) (Job, error) {
	found, err := m.Find(jobID)
	if err != nil {
		return Job{}, err
	}
	if found.CurrentChange() == nil {
		return Job{}, ErrNoCurrentChange
	}
	if now.IsZero() {
		now = time.Now()
	}
	if commit.CreatedAt.IsZero() {
		commit.CreatedAt = now
	}

	tx, err := m.sqlDB.Begin()
	if err != nil {
		return Job{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var changeRowID int64
	err = tx.QueryRow(`SELECT id FROM job_changes WHERE repo = ? AND job_id = ? ORDER BY position DESC LIMIT 1;`,
		found.Repo,
		found.ID,
	).Scan(&changeRowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrNoCurrentChange
		}
		return Job{}, err
	}

	var count int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM job_commits WHERE job_change_id = ?;`, changeRowID).Scan(&count); err != nil {
		return Job{}, err
	}

	reviewOutcome, reviewComments, reviewAgentID, reviewReviewedAt := reviewFields(commit.Review)
	if err = ensureAgentSession(tx, found.Repo, commit.AgentSessionID, now); err != nil {
		return Job{}, err
	}
	if _, err = tx.Exec(`INSERT INTO job_commits (
		job_change_id, commit_id, draft_message, tests_passed, agent_session_id,
		review_outcome, review_comments, review_agent_session_id, review_reviewed_at,
		created_at, position
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		changeRowID,
		commit.CommitID,
		commit.DraftMessage,
		boolPointerToSQLite(commit.TestsPassed),
		commit.AgentSessionID,
		reviewOutcome,
		reviewComments,
		reviewAgentID,
		reviewReviewedAt,
		formatJobTime(commit.CreatedAt),
		count,
	); err != nil {
		return Job{}, err
	}

	if _, err = tx.Exec(`UPDATE jobs SET updated_at = ? WHERE repo = ? AND id = ?;`,
		formatJobTime(now),
		found.Repo,
		found.ID,
	); err != nil {
		return Job{}, err
	}

	if err = tx.Commit(); err != nil {
		return Job{}, err
	}

	return m.loadJobByID(found.Repo, found.ID)
}

// UpdateCurrentCommit updates the current in-progress commit.
// Returns ErrNoCurrentChange if there are no changes, or if the last change is complete.
// Returns ErrNoCurrentCommit if the current change has no commits.
func (m *Manager) UpdateCurrentCommit(jobID string, update JobCommitUpdate, now time.Time) (Job, error) {
	found, err := m.Find(jobID)
	if err != nil {
		return Job{}, err
	}
	if found.CurrentChange() == nil {
		return Job{}, ErrNoCurrentChange
	}
	if now.IsZero() {
		now = time.Now()
	}

	tx, err := m.sqlDB.Begin()
	if err != nil {
		return Job{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var changeRowID int64
	err = tx.QueryRow(`SELECT id FROM job_changes WHERE repo = ? AND job_id = ? ORDER BY position DESC LIMIT 1;`,
		found.Repo,
		found.ID,
	).Scan(&changeRowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrNoCurrentChange
		}
		return Job{}, err
	}

	var commitRowID int64
	var testsPassed sql.NullInt64
	var reviewOutcome sql.NullString
	var reviewComments string
	var reviewAgentID string
	var reviewReviewedAt string
	var agentSessionID string
	if err = tx.QueryRow(`SELECT id, tests_passed, review_outcome, review_comments,
		review_agent_session_id, review_reviewed_at, agent_session_id
		FROM job_commits WHERE job_change_id = ? ORDER BY position DESC LIMIT 1;`,
		changeRowID,
	).Scan(&commitRowID, &testsPassed, &reviewOutcome, &reviewComments, &reviewAgentID, &reviewReviewedAt, &agentSessionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrNoCurrentCommit
		}
		return Job{}, err
	}

	if strings.TrimSpace(agentSessionID) != "" {
		if err = ensureAgentSession(tx, found.Repo, agentSessionID, now); err != nil {
			return Job{}, err
		}
	}

	if update.TestsPassed != nil {
		if *update.TestsPassed {
			testsPassed = sql.NullInt64{Int64: 1, Valid: true}
		} else {
			testsPassed = sql.NullInt64{Int64: 0, Valid: true}
		}
	}
	if update.Review != nil {
		review := *update.Review
		if review.ReviewedAt.IsZero() {
			review.ReviewedAt = now
		}
		reviewOutcome = sql.NullString{String: string(review.Outcome), Valid: true}
		reviewComments = review.Comments
		reviewAgentID = review.AgentSessionID
		reviewReviewedAt = formatOptionalJobTime(review.ReviewedAt)
		if err = ensureAgentSession(tx, found.Repo, review.AgentSessionID, now); err != nil {
			return Job{}, err
		}
	}

	if _, err = tx.Exec(`UPDATE job_commits SET tests_passed = ?, review_outcome = ?, review_comments = ?,
		review_agent_session_id = ?, review_reviewed_at = ? WHERE id = ?;`,
		sqlNullInt64(testsPassed),
		sqlNullString(reviewOutcome),
		reviewComments,
		reviewAgentID,
		reviewReviewedAt,
		commitRowID,
	); err != nil {
		return Job{}, err
	}

	if _, err = tx.Exec(`UPDATE jobs SET updated_at = ? WHERE repo = ? AND id = ?;`,
		formatJobTime(now),
		found.Repo,
		found.ID,
	); err != nil {
		return Job{}, err
	}

	if err = tx.Commit(); err != nil {
		return Job{}, err
	}

	return m.loadJobByID(found.Repo, found.ID)
}

// SetProjectReview sets the project's final review on the job.
func (m *Manager) SetProjectReview(jobID string, review JobReview, now time.Time) (Job, error) {
	found, err := m.Find(jobID)
	if err != nil {
		return Job{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	if review.ReviewedAt.IsZero() {
		review.ReviewedAt = now
	}

	outcome, comments, agentID, reviewedAt := reviewFields(&review)
	if err := ensureAgentSession(m.sqlDB, found.Repo, review.AgentSessionID, now); err != nil {
		return Job{}, err
	}
	if _, err := m.sqlDB.Exec(`UPDATE jobs SET project_review_outcome = ?, project_review_comments = ?,
		project_review_agent_session_id = ?, project_review_reviewed_at = ?, updated_at = ?
		WHERE repo = ? AND id = ?;`,
		outcome,
		comments,
		agentID,
		reviewedAt,
		formatJobTime(now),
		found.Repo,
		found.ID,
	); err != nil {
		return Job{}, err
	}

	return m.loadJobByID(found.Repo, found.ID)
}

type dbExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func agentSessionExists(sessions []JobAgentSession, sessionID string) bool {
	if strings.TrimSpace(sessionID) == "" {
		return false
	}
	for _, session := range sessions {
		if session.ID == sessionID {
			return true
		}
	}
	return false
}

func ensureAgentSession(exec dbExecutor, repo, sessionID string, now time.Time) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if exec == nil {
		return fmt.Errorf("agent session insert: exec is nil")
	}
	createdAt := formatJobTime(now)
	_, err := exec.Exec(`INSERT OR IGNORE INTO agent_sessions (
		repo, id, status, model, created_at, started_at, updated_at, completed_at,
		exit_code, duration_seconds, tokens_used, cost
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		repo,
		sessionID,
		string(StatusActive),
		"",
		createdAt,
		"",
		createdAt,
		"",
		nil,
		0,
		0,
		0,
	)
	return err
}

// ListFilter configures which jobs to return.
type ListFilter struct {
	// Status filters by exact status match.
	Status *Status
	// IncludeAll includes jobs regardless of status.
	IncludeAll bool
}

// List returns jobs for the repo.
func (m *Manager) List(filter ListFilter) ([]Job, error) {
	if filter.Status != nil {
		normalized := normalizeStatus(*filter.Status)
		filter.Status = &normalized
		if !filter.Status.IsValid() {
			return nil, formatInvalidStatusError(*filter.Status)
		}
	}

	repoName, err := db.GetOrCreateRepoName(m.sqlDB, m.repoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	query := "SELECT id FROM jobs WHERE repo = ?"
	args := []any{repoName}
	if filter.Status != nil {
		query += " AND status = ?"
		args = append(args, string(*filter.Status))
	} else if !filter.IncludeAll {
		query += " AND status = ?"
		args = append(args, string(StatusActive))
	}
	query += " ORDER BY started_at, id;"

	rows, err := m.sqlDB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	items := make([]Job, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list jobs: %w", err)
		}
		job, err := m.loadJobByID(repoName, id)
		if err != nil {
			return nil, err
		}
		items = append(items, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	return items, nil
}

// Find returns the job with the given id or prefix for the repo.
func (m *Manager) Find(jobID string) (Job, error) {
	if jobID == "" {
		return Job{}, ErrJobNotFound
	}

	repoName, err := db.GetOrCreateRepoName(m.sqlDB, m.repoPath)
	if err != nil {
		return Job{}, fmt.Errorf("get repo name: %w", err)
	}

	rows, err := m.sqlDB.Query("SELECT id FROM jobs WHERE repo = ? ORDER BY id;", repoName)
	if err != nil {
		return Job{}, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	jobIDs := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return Job{}, fmt.Errorf("list jobs: %w", err)
		}
		jobIDs = append(jobIDs, id)
	}
	if err := rows.Err(); err != nil {
		return Job{}, fmt.Errorf("list jobs: %w", err)
	}

	matchID, found, ambiguous := ids.MatchPrefix(jobIDs, jobID)
	if ambiguous {
		return Job{}, fmt.Errorf("%w: %s", ErrAmbiguousJobIDPrefix, jobID)
	}
	if !found {
		return Job{}, ErrJobNotFound
	}

	return m.loadJobByID(repoName, matchID)
}

// MarkStaleJobsFailed finds active jobs that haven't been updated within the
// StaleJobTimeout and marks them as failed. Returns the number of jobs marked.
func (m *Manager) MarkStaleJobsFailed(now time.Time) (int, error) {
	repoName, err := db.GetOrCreateRepoName(m.sqlDB, m.repoPath)
	if err != nil {
		return 0, fmt.Errorf("get repo name: %w", err)
	}

	cutoff := formatJobTime(now.Add(-StaleJobTimeout))
	result, err := m.sqlDB.Exec(`UPDATE jobs
		SET status = ?, completed_at = ?, updated_at = ?
		WHERE repo = ? AND status = ? AND updated_at <= ?;`,
		string(StatusFailed),
		formatOptionalJobTime(now),
		formatJobTime(now),
		repoName,
		string(StatusActive),
		cutoff,
	)
	if err != nil {
		return 0, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(rows), nil
}

// IsJobStale returns true if the job is active but hasn't been updated within
// the StaleJobTimeout.
func IsJobStale(job Job, now time.Time) bool {
	if job.Status != StatusActive {
		return false
	}
	cutoff := now.Add(-StaleJobTimeout)
	return !job.UpdatedAt.After(cutoff)
}

// CountByHabit returns a map of habit name to job count for all habits in the repo.
// Jobs for habits have TodoID formatted as "habit:<name>".
func (m *Manager) CountByHabit() (map[string]int, error) {
	repoName, err := db.GetOrCreateRepoName(m.sqlDB, m.repoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	rows, err := m.sqlDB.Query(`SELECT todo_id FROM jobs WHERE repo = ? AND todo_id LIKE ?;`, repoName, "habit:%")
	if err != nil {
		return nil, fmt.Errorf("count by habit: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	const habitPrefix = "habit:"
	for rows.Next() {
		var todoID string
		if err := rows.Scan(&todoID); err != nil {
			return nil, fmt.Errorf("count by habit: %w", err)
		}
		if !strings.HasPrefix(todoID, habitPrefix) {
			continue
		}
		habitName := strings.TrimPrefix(todoID, habitPrefix)
		counts[habitName]++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("count by habit: %w", err)
	}

	return counts, nil
}

func resolveDB(opts OpenOptions) (*sql.DB, func() error, error) {
	if opts.DB != nil {
		return opts.DB, func() error { return nil }, nil
	}

	stateDir, err := paths.ResolveWithDefault(opts.StateDir, paths.DefaultStateDir)
	if err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create state dir: %w", err)
	}

	path := filepath.Join(stateDir, "state.db")
	store, err := db.Open(path, db.OpenOptions{LegacyJSONPath: filepath.Join(stateDir, "state.json")})
	if err != nil {
		return nil, nil, err
	}

	return store.SqlDB(), store.Close, nil
}

func (m *Manager) loadJobByID(repoName, jobID string) (Job, error) {
	row := m.sqlDB.QueryRow(`SELECT todo_id, agent, implementation_model, code_review_model,
		project_review_model, stage, status, feedback,
		project_review_outcome, project_review_comments, project_review_agent_session_id,
		project_review_reviewed_at, created_at, started_at, updated_at, completed_at
		FROM jobs WHERE repo = ? AND id = ?;`, repoName, jobID)

	var item Job
	item.ID = jobID
	item.Repo = repoName
	var stage string
	var status string
	var projectOutcome sql.NullString
	var projectComments string
	var projectAgentID string
	var projectReviewedAt string
	var createdAt string
	var startedAt string
	var updatedAt string
	var completedAt string
	if err := row.Scan(
		&item.TodoID,
		&item.Agent,
		&item.ImplementationModel,
		&item.CodeReviewModel,
		&item.ProjectReviewModel,
		&stage,
		&status,
		&item.Feedback,
		&projectOutcome,
		&projectComments,
		&projectAgentID,
		&projectReviewedAt,
		&createdAt,
		&startedAt,
		&updatedAt,
		&completedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrJobNotFound
		}
		return Job{}, fmt.Errorf("load job: %w", err)
	}

	item.Stage = Stage(stage)
	item.Status = Status(status)
	if item.Stage == "" {
		item.Stage = StageImplementing
	}
	if item.Status == "" {
		item.Status = StatusActive
	}

	parsedCreatedAt, err := parseJobTime(createdAt)
	if err != nil {
		return Job{}, fmt.Errorf("load job created_at: %w", err)
	}
	parsedStartedAt, err := parseJobTime(startedAt)
	if err != nil {
		return Job{}, fmt.Errorf("load job started_at: %w", err)
	}
	parsedUpdatedAt, err := parseJobTime(updatedAt)
	if err != nil {
		return Job{}, fmt.Errorf("load job updated_at: %w", err)
	}
	parsedCompletedAt, err := parseJobTime(completedAt)
	if err != nil {
		return Job{}, fmt.Errorf("load job completed_at: %w", err)
	}
	item.CreatedAt = parsedCreatedAt
	item.StartedAt = parsedStartedAt
	item.UpdatedAt = parsedUpdatedAt
	item.CompletedAt = parsedCompletedAt

	if projectOutcome.Valid && projectOutcome.String != "" {
		reviewedAt, err := parseJobTime(projectReviewedAt)
		if err != nil {
			return Job{}, fmt.Errorf("load project review reviewed_at: %w", err)
		}
		item.ProjectReview = &JobReview{
			Outcome:        ReviewOutcome(projectOutcome.String),
			Comments:       projectComments,
			AgentSessionID: projectAgentID,
			ReviewedAt:     reviewedAt,
		}
	}

	sessions, err := m.loadJobSessions(repoName, jobID)
	if err != nil {
		return Job{}, err
	}
	item.AgentSessions = sessions

	changes, err := m.loadJobChanges(repoName, jobID)
	if err != nil {
		return Job{}, err
	}
	item.Changes = changes

	return item, nil
}

func (m *Manager) loadJobSessions(repoName, jobID string) ([]JobAgentSession, error) {
	rows, err := m.sqlDB.Query(`SELECT session_id, purpose FROM job_agent_sessions
		WHERE repo = ? AND job_id = ? ORDER BY position;`, repoName, jobID)
	if err != nil {
		return nil, fmt.Errorf("load job sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]JobAgentSession, 0)
	for rows.Next() {
		var sessionID string
		var purpose string
		if err := rows.Scan(&sessionID, &purpose); err != nil {
			return nil, fmt.Errorf("load job sessions: %w", err)
		}
		sessions = append(sessions, JobAgentSession{ID: sessionID, Purpose: purpose})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load job sessions: %w", err)
	}
	return sessions, nil
}

func (m *Manager) loadJobChanges(repoName, jobID string) ([]JobChange, error) {
	rows, err := m.sqlDB.Query(`SELECT id, change_id, created_at FROM job_changes
		WHERE repo = ? AND job_id = ? ORDER BY position;`, repoName, jobID)
	if err != nil {
		return nil, fmt.Errorf("load job changes: %w", err)
	}
	defer rows.Close()

	changes := make([]JobChange, 0)
	for rows.Next() {
		var rowID int64
		var changeID string
		var createdAt string
		if err := rows.Scan(&rowID, &changeID, &createdAt); err != nil {
			return nil, fmt.Errorf("load job changes: %w", err)
		}
		parsedCreatedAt, err := parseJobTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("load job change created_at: %w", err)
		}
		commits, err := m.loadJobCommits(rowID)
		if err != nil {
			return nil, err
		}
		changes = append(changes, JobChange{
			ChangeID:  changeID,
			CreatedAt: parsedCreatedAt,
			Commits:   commits,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load job changes: %w", err)
	}

	return changes, nil
}

func (m *Manager) loadJobCommits(changeRowID int64) ([]JobCommit, error) {
	rows, err := m.sqlDB.Query(`SELECT commit_id, draft_message, tests_passed, agent_session_id,
		review_outcome, review_comments, review_agent_session_id, review_reviewed_at,
		created_at FROM job_commits WHERE job_change_id = ? ORDER BY position;`, changeRowID)
	if err != nil {
		return nil, fmt.Errorf("load job commits: %w", err)
	}
	defer rows.Close()

	commits := make([]JobCommit, 0)
	for rows.Next() {
		var commit JobCommit
		var testsPassed sql.NullInt64
		var reviewOutcome sql.NullString
		var reviewComments string
		var reviewAgentID string
		var reviewReviewedAt string
		var createdAt string
		if err := rows.Scan(
			&commit.CommitID,
			&commit.DraftMessage,
			&testsPassed,
			&commit.AgentSessionID,
			&reviewOutcome,
			&reviewComments,
			&reviewAgentID,
			&reviewReviewedAt,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("load job commits: %w", err)
		}
		if testsPassed.Valid {
			value := testsPassed.Int64 == 1
			commit.TestsPassed = &value
		}
		if reviewOutcome.Valid && reviewOutcome.String != "" {
			reviewedAt, err := parseJobTime(reviewReviewedAt)
			if err != nil {
				return nil, fmt.Errorf("load job commit review reviewed_at: %w", err)
			}
			commit.Review = &JobReview{
				Outcome:        ReviewOutcome(reviewOutcome.String),
				Comments:       reviewComments,
				AgentSessionID: reviewAgentID,
				ReviewedAt:     reviewedAt,
			}
		}
		parsedCreatedAt, err := parseJobTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("load job commit created_at: %w", err)
		}
		commit.CreatedAt = parsedCreatedAt
		commits = append(commits, commit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load job commits: %w", err)
	}

	return commits, nil
}

func formatJobTime(value time.Time) string {
	if value.IsZero() {
		return time.Time{}.UTC().Format(time.RFC3339Nano)
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func formatOptionalJobTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseJobTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func reviewFields(review *JobReview) (any, string, string, string) {
	if review == nil {
		return nil, "", "", ""
	}
	return string(review.Outcome), review.Comments, review.AgentSessionID, formatOptionalJobTime(review.ReviewedAt)
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

func sqlNullInt64(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func sqlNullString(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

