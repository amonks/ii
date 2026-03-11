package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	internalagent "monks.co/pkg/agent"
	"monks.co/incrementum/internal/agents"
	"monks.co/incrementum/internal/config"
	"monks.co/incrementum/internal/db"
	internalids "monks.co/incrementum/internal/ids"
	"monks.co/incrementum/internal/paths"
	"monks.co/pkg/llm"
)

// Store provides access to agent functionality with session persistence.
type Store struct {
	// closeDB closes any owned database connection.
	closeDB func() error
	// sqlDB is the sqlite handle used for persistence.
	sqlDB *sql.DB
	registry  *agents.ModelRegistry
	config    *config.Config
}

// SetCloseFunc configures the close callback for the store.
func (s *Store) SetCloseFunc(closeFn func() error) {
	if s == nil {
		return
	}
	s.closeDB = closeFn
}

// Options configures how the store is opened.
type Options struct {
	// StateDir is the directory for state files.
	// Default: ~/.local/state/incrementum
	StateDir string

	// DB is an existing SQLite database connection to use.
	// If set, StateDir is ignored for persistence.
	DB *sql.DB

	// RepoPath is the repository path for loading project-specific config.
	// If empty, only global config is loaded.
	RepoPath string
}

// OpenWithDB opens an agent store using an existing SQLite handle.
func OpenWithDB(dbHandle *sql.DB, opts Options) (*Store, error) {
	opts.DB = dbHandle
	return OpenWithOptions(opts)
}

// Only loads global configuration (no project-specific config).
func Open() (*Store, error) {
	return OpenWithOptions(Options{})
}

// OpenWithOptions opens the agent store with the given options.
func OpenWithOptions(opts Options) (*Store, error) {
	stateDir := opts.StateDir
	if opts.DB == nil {
		var err error
		stateDir, err = paths.ResolveWithDefault(opts.StateDir, paths.DefaultStateDir)
		if err != nil {
			return nil, fmt.Errorf("resolve state dir: %w", err)
		}
	}

	// Load config
	var cfg *config.Config
	var err error
	if opts.RepoPath != "" {
		cfg, err = config.Load(opts.RepoPath)
	} else {
		cfg, err = config.LoadGlobal()
	}
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Build model registry from config
	registry, err := agents.NewModelRegistry(cfg)
	if err != nil {
		return nil, fmt.Errorf("build model registry: %w", err)
	}

	sqlDB, closeFn, err := openAgentDB(opts, stateDir)
	if err != nil {
		return nil, err
	}

	return &Store{
		sqlDB:    sqlDB,
		closeDB:  closeFn,
		registry: registry,
		config:   cfg,
	}, nil
}

func openAgentDB(opts Options, stateDir string) (*sql.DB, func() error, error) {
	if opts.DB != nil {
		return opts.DB, func() error { return nil }, nil
	}

	dbPath, err := resolveDBPath(stateDir)
	if err != nil {
		return nil, nil, err
	}

	store, err := db.Open(dbPath, db.OpenOptions{LegacyJSONPath: filepath.Join(stateDir, "state.json")})
	if err != nil {
		return nil, nil, err
	}

	return store.SqlDB(), store.Close, nil
}

func resolveDBPath(stateDir string) (string, error) {
	if stateDir != "" {
		return filepath.Join(stateDir, "state.db"), nil
	}
	return paths.DefaultDBPath()
}

// Close closes any owned database connection.
func (s *Store) Close() error {
	if s == nil || s.closeDB == nil {
		return nil
	}
	return s.closeDB()
}

// RepoNameForPath returns the repo name for the given path, if present.
func (s *Store) RepoNameForPath(path string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("repo name for path: store is nil")
	}
	return db.RepoNameForPath(s.sqlDB, path)
}

// GetOrCreateRepoName returns the repo name for the given path, creating one if needed.
func (s *Store) GetOrCreateRepoName(path string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("get repo name: store is nil")
	}
	return db.GetOrCreateRepoName(s.sqlDB, path)
}

// ResolveModel resolves a model using the priority chain:
// 1. Explicit model (if provided)
// 2. INCREMENTUM_AGENT_MODEL environment variable
// 3. Per-task model (taskModel parameter, e.g., job.implementation-model)
// 4. agent.model from config
// 5. llm.model from config (final fallback)
// 6. Error if no model resolved
func (s *Store) ResolveModel(explicit string, taskModel string) (llm.Model, error) {
	// Priority 1: Explicit model
	if explicit != "" {
		return s.registry.GetModel(explicit)
	}

	// Priority 2: Environment variable
	if envModel := os.Getenv("INCREMENTUM_AGENT_MODEL"); envModel != "" {
		return s.registry.GetModel(envModel)
	}

	// Priority 3: Per-task model
	if taskModel != "" {
		return s.registry.GetModel(taskModel)
	}

	// Priority 4: agent.model from config
	if s.config != nil && s.config.Agent.Model != "" {
		return s.registry.GetModel(s.config.Agent.Model)
	}

	// Priority 5: llm.model from config (final fallback)
	if defaultModel := s.registry.DefaultModel(); defaultModel != "" {
		return s.registry.GetModel(defaultModel)
	}

	return llm.Model{}, fmt.Errorf("%w: specify --model, set INCREMENTUM_AGENT_MODEL, or configure agent.model or llm.model in config", ErrNoModelConfigured)
}

// ResolveImplementationModel resolves a model for implementation tasks.
// Priority: explicit || job.implementation-model || agent.model || llm.model
func (s *Store) ResolveImplementationModel(explicit string) (llm.Model, error) {
	taskModel := ""
	if s.config != nil {
		taskModel = s.config.Job.ImplementationModel
	}
	return s.ResolveModel(explicit, taskModel)
}

// ResolveCodeReviewModel resolves a model for code review tasks.
// Priority: explicit || job.code-review-model || agent.model || llm.model
func (s *Store) ResolveCodeReviewModel(explicit string) (llm.Model, error) {
	taskModel := ""
	if s.config != nil {
		taskModel = s.config.Job.CodeReviewModel
	}
	return s.ResolveModel(explicit, taskModel)
}

// ResolveProjectReviewModel resolves a model for project review tasks.
// Priority: explicit || job.project-review-model || agent.model || llm.model
func (s *Store) ResolveProjectReviewModel(explicit string) (llm.Model, error) {
	taskModel := ""
	if s.config != nil {
		taskModel = s.config.Job.ProjectReviewModel
	}
	return s.ResolveModel(explicit, taskModel)
}

// Run starts an agent run with the given options.
// It returns a RunHandle that provides access to events and the final result.
func (s *Store) Run(ctx context.Context, opts RunOptions) (*RunHandle, error) {
	// Resolve model
	model, err := s.ResolveModel(opts.Model, "")
	if err != nil {
		return nil, fmt.Errorf("resolve model: %w", err)
	}

	// Get repo name for session storage
	repoName, err := s.GetOrCreateRepoName(opts.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	// Generate session ID
	now := opts.StartedAt
	if now.IsZero() {
		now = time.Now()
	}
	sessionSeed := opts.RepoPath + opts.Prompt.UserContent
	sessionID := internalids.GenerateWithTimestamp(sessionSeed, now, internalids.DefaultLength)

	// Create initial session record
	session := Session{
		ID:        sessionID,
		Repo:      repoName,
		Status:    SessionActive,
		Model:     model.ID,
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}

	if err := s.saveSession(session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	// Resolve global config directory for AGENTS.md support
	globalConfigDir, err := paths.DefaultConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve global config dir: %w", err)
	}

	// Configure agent
	agentConfig := AgentConfig{
		Model:           model,
		Permissions:     defaultBashPermissions(),
		WorkDir:         opts.WorkDir,
		GlobalConfigDir: globalConfigDir,
		Env:             opts.Env,
		InputCh:         opts.InputCh,
		SessionID:       sessionID,
		Version:         opts.Version,
		CacheRetention:  llm.CacheRetention(s.config.Agent.CacheRetention),
	}

	// Start internal agent
	internalHandle, err := internalagent.Run(ctx, opts.Prompt, agentConfig)
	if err != nil {
		// Mark session as failed
		session.Status = SessionFailed
		session.UpdatedAt = time.Now()
		s.saveSession(session)
		return nil, fmt.Errorf("start agent: %w", err)
	}

	// Create result channel
	resultCh := make(chan RunResult, 1)

	handle := &RunHandle{
		sessionID: sessionID,
		handle:    internalHandle,
		resultCh:  resultCh,
	}

	// Start result collection goroutine
	go s.collectResult(internalHandle, resultCh, session, model.ContextWindow)

	return handle, nil
}

// collectResult waits for the agent to complete and updates session state.
func (s *Store) collectResult(
	internalHandle *internalagent.RunHandle,
	resultCh chan<- RunResult,
	session Session,
	contextWindow int,
) {
	defer close(resultCh)

	// Wait for final result
	internalResult, _ := internalHandle.Wait()

	// Build result
	result := RunResult{
		SessionID:     session.ID,
		ExitCode:      0,
		Messages:      internalResult.Messages,
		Usage:         internalResult.Usage,
		ContextWindow: contextWindow,
	}

	// Update session based on result
	now := time.Now()
	session.UpdatedAt = now
	session.CompletedAt = now
	session.DurationSeconds = int(now.Sub(session.StartedAt).Seconds())
	session.TokensUsed = internalResult.Usage.Total
	session.Cost = internalResult.Usage.Cost.Total

	// internalResult.Error is the authoritative error source for the agent run.
	// The Wait() return error is the same value (Wait returns result, result.Error).
	if internalResult.Error != nil {
		session.Status = SessionFailed
		exitCode := 1
		session.ExitCode = &exitCode
		result.ExitCode = 1
		result.Error = internalResult.Error.Error()
	} else {
		session.Status = SessionCompleted
		exitCode := 0
		session.ExitCode = &exitCode
	}

	// Save final session state
	s.saveSession(session)

	resultCh <- result
}

// ListSessions returns all sessions for the given repository.
func (s *Store) ListSessions(repoPath string) ([]Session, error) {
	repoName, err := s.getRepoNameIfExists(repoPath)
	if err != nil {
		return nil, err
	}
	if repoName == "" {
		return nil, nil // No sessions for unknown repo
	}

	sessions, err := s.listSessionsByRepo(repoName)
	if err != nil {
		return nil, err
	}

	// Sort by created time, most recent first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	return sessions, nil
}

// FindSession returns the session with the given ID.
// The ID can be a prefix if it uniquely identifies a session.
func (s *Store) FindSession(repoPath, sessionID string) (Session, error) {
	repoName, err := s.getRepoNameIfExists(repoPath)
	if err != nil {
		return Session{}, err
	}
	if repoName == "" {
		return Session{}, fmt.Errorf("session not found: %s", sessionID)
	}

	// Try exact match first.
	if session, found, err := s.findSessionByID(repoName, sessionID); err != nil {
		return Session{}, err
	} else if found {
		return session, nil
	}

	// Some call sites may pass the full state key as the session ID ("<repo>/<id>").
	// Accept that form as well by attempting a lookup when a repo-prefixed value is used.
	if after, ok := strings.CutPrefix(sessionID, repoName+"/"); ok {
		trimmed := after
		if session, found, err := s.findSessionByID(repoName, trimmed); err != nil {
			return Session{}, err
		} else if found {
			return session, nil
		}
	}

	// Try prefix match (case-insensitive)
	return s.findSessionByPrefix(repoName, sessionID)
}

func (s *Store) saveSession(session Session) error {
	if s == nil || s.sqlDB == nil {
		return fmt.Errorf("save session: db is nil")
	}

	_, err := s.sqlDB.Exec(`INSERT INTO agent_sessions (
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
		session.Repo,
		session.ID,
		string(session.Status),
		session.Model,
		formatSessionTime(session.CreatedAt),
		formatOptionalSessionTime(session.StartedAt),
		formatSessionTime(session.UpdatedAt),
		formatOptionalSessionTime(session.CompletedAt),
		sqlNullIntPointer(session.ExitCode),
		session.DurationSeconds,
		session.TokensUsed,
		session.Cost,
	)
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}

func (s *Store) listSessionsByRepo(repoName string) ([]Session, error) {
	if s == nil || s.sqlDB == nil {
		return nil, fmt.Errorf("list sessions: db is nil")
	}

	rows, err := s.sqlDB.Query(`SELECT id, status, model, created_at, started_at, updated_at,
		completed_at, exit_code, duration_seconds, tokens_used, cost
		FROM agent_sessions WHERE repo = ?;`, repoName)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		session, err := scanSessionRows(rows, repoName)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return sessions, nil
}

func (s *Store) findSessionByID(repoName, sessionID string) (Session, bool, error) {
	if s == nil || s.sqlDB == nil {
		return Session{}, false, fmt.Errorf("find session: db is nil")
	}

	row := s.sqlDB.QueryRow(`SELECT id, status, model, created_at, started_at, updated_at,
		completed_at, exit_code, duration_seconds, tokens_used, cost
		FROM agent_sessions WHERE repo = ? AND id = ?;`, repoName, sessionID)
	session, err := scanSessionRow(row, repoName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, false, nil
		}
		return Session{}, false, fmt.Errorf("find session: %w", err)
	}
	return session, true, nil
}

func (s *Store) findSessionByPrefix(repoName, prefix string) (Session, error) {
	if s == nil || s.sqlDB == nil {
		return Session{}, fmt.Errorf("find session: db is nil")
	}

	rows, err := s.sqlDB.Query(`SELECT id, status, model, created_at, started_at, updated_at,
		completed_at, exit_code, duration_seconds, tokens_used, cost
		FROM agent_sessions WHERE repo = ? AND lower(id) LIKE ?
		ORDER BY id;`, repoName, strings.ToLower(prefix)+"%")
	if err != nil {
		return Session{}, fmt.Errorf("find session: %w", err)
	}
	defer rows.Close()

	var matches []Session
	for rows.Next() {
		session, err := scanSessionRows(rows, repoName)
		if err != nil {
			return Session{}, err
		}
		matches = append(matches, session)
	}
	if err := rows.Err(); err != nil {
		return Session{}, fmt.Errorf("find session: %w", err)
	}

	if len(matches) == 0 {
		return Session{}, fmt.Errorf("session not found: %s", prefix)
	}
	if len(matches) > 1 {
		return Session{}, fmt.Errorf("ambiguous session ID: %s matches %d sessions", prefix, len(matches))
	}
	return matches[0], nil
}

func scanSessionRow(row *sql.Row, repoName string) (Session, error) {
	var session Session
	var status string
	var createdAt string
	var startedAt string
	var updatedAt string
	var completedAt string
	var exitCode sql.NullInt64
	if err := row.Scan(
		&session.ID,
		&status,
		&session.Model,
		&createdAt,
		&startedAt,
		&updatedAt,
		&completedAt,
		&exitCode,
		&session.DurationSeconds,
		&session.TokensUsed,
		&session.Cost,
	); err != nil {
		return Session{}, err
	}
	parsed, err := hydrateSession(repoName, session.ID, status, session.Model, createdAt, startedAt, updatedAt, completedAt, exitCode, session.DurationSeconds, session.TokensUsed, session.Cost)
	if err != nil {
		return Session{}, err
	}
	return parsed, nil
}

func scanSessionRows(rows *sql.Rows, repoName string) (Session, error) {
	var sessionID string
	var status string
	var model string
	var createdAt string
	var startedAt string
	var updatedAt string
	var completedAt string
	var exitCode sql.NullInt64
	var duration int
	var tokens int
	var cost float64
	if err := rows.Scan(
		&sessionID,
		&status,
		&model,
		&createdAt,
		&startedAt,
		&updatedAt,
		&completedAt,
		&exitCode,
		&duration,
		&tokens,
		&cost,
	); err != nil {
		return Session{}, fmt.Errorf("scan session: %w", err)
	}
	return hydrateSession(repoName, sessionID, status, model, createdAt, startedAt, updatedAt, completedAt, exitCode, duration, tokens, cost)
}

func hydrateSession(repoName, id, status, model, createdAt, startedAt, updatedAt, completedAt string, exitCode sql.NullInt64, duration, tokens int, cost float64) (Session, error) {
	createdAtTime, err := parseSessionTime(createdAt)
	if err != nil {
		return Session{}, fmt.Errorf("scan session created_at: %w", err)
	}
	startedAtTime, err := parseOptionalSessionTime(startedAt)
	if err != nil {
		return Session{}, fmt.Errorf("scan session started_at: %w", err)
	}
	updatedAtTime, err := parseSessionTime(updatedAt)
	if err != nil {
		return Session{}, fmt.Errorf("scan session updated_at: %w", err)
	}
	completedAtTime, err := parseOptionalSessionTime(completedAt)
	if err != nil {
		return Session{}, fmt.Errorf("scan session completed_at: %w", err)
	}

	session := Session{
		ID:              id,
		Repo:            repoName,
		Status:          SessionStatus(status),
		Model:           model,
		CreatedAt:       createdAtTime,
		StartedAt:       startedAtTime,
		UpdatedAt:       updatedAtTime,
		CompletedAt:     completedAtTime,
		DurationSeconds: duration,
		TokensUsed:      tokens,
		Cost:            cost,
	}
	if exitCode.Valid {
		exit := int(exitCode.Int64)
		session.ExitCode = &exit
	}
	if session.Status == "" {
		session.Status = SessionActive
	}
	return session, nil
}

func parseSessionTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func parseOptionalSessionTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func formatSessionTime(value time.Time) string {
	if value.IsZero() {
		return time.Time{}.UTC().Format(time.RFC3339Nano)
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func formatOptionalSessionTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func sqlNullIntPointer(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

// getRepoNameIfExists gets the repo name if it exists, or returns empty string.
func (s *Store) getRepoNameIfExists(repoPath string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("get repo name: store is nil")
	}
	return s.RepoNameForPath(repoPath)
}

// defaultBashPermissions returns the default bash permissions.
func defaultBashPermissions() BashPermissions {
	return BashPermissions{
		Rules: []BashRule{
			{Pattern: "jj diff", Allow: true},
			{Pattern: "jj diff *", Allow: true},
			{Pattern: "jj file", Allow: true},
			{Pattern: "jj file *", Allow: true},
			{Pattern: "jj log", Allow: true},
			{Pattern: "jj log *", Allow: true},
			{Pattern: "jj show", Allow: true},
			{Pattern: "jj show *", Allow: true},
			{Pattern: "jj status", Allow: true},
			{Pattern: "jj status *", Allow: true},
			{Pattern: "jj *", Allow: false},
			{Pattern: "git *", Allow: false},
			{Pattern: "ii todo create *", Allow: true},
			{Pattern: "ii todo show *", Allow: true},
			{Pattern: "ii *", Allow: false},
			{Pattern: "*", Allow: true},
		},
	}
}
