package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	internalagent "github.com/amonks/incrementum/internal/agent"
	"github.com/amonks/incrementum/internal/config"
	internalids "github.com/amonks/incrementum/internal/ids"
	"github.com/amonks/incrementum/internal/paths"
	"github.com/amonks/incrementum/internal/state"
	"github.com/amonks/incrementum/llm"
)

// Store provides access to agent functionality with session persistence
// and event logging.
type Store struct {
	stateStore *state.Store
	eventsDir  string
	llmStore   *llm.Store
	config     *config.Config
}

// Options configures how the store is opened.
type Options struct {
	// StateDir is the directory for state files.
	// Default: ~/.local/state/incrementum
	StateDir string

	// EventsDir is the directory for event logs.
	// Default: ~/.local/share/incrementum/agent/events
	EventsDir string

	// RepoPath is the repository path for loading project-specific config.
	// If empty, only global config is loaded.
	RepoPath string
}

// Open opens the agent store with default options.
// Only loads global configuration (no project-specific config).
func Open() (*Store, error) {
	return OpenWithOptions(Options{})
}

// OpenWithOptions opens the agent store with the given options.
func OpenWithOptions(opts Options) (*Store, error) {
	stateDir, err := paths.ResolveWithDefault(opts.StateDir, paths.DefaultStateDir)
	if err != nil {
		return nil, fmt.Errorf("resolve state dir: %w", err)
	}

	eventsDir, err := paths.ResolveWithDefault(opts.EventsDir, defaultEventsDir)
	if err != nil {
		return nil, fmt.Errorf("resolve events dir: %w", err)
	}

	// Load config
	var cfg *config.Config
	if opts.RepoPath != "" {
		cfg, err = config.Load(opts.RepoPath)
	} else {
		cfg, err = config.LoadGlobal()
	}
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Open LLM store for model resolution
	llmStore, err := llm.OpenWithOptions(llm.Options{
		RepoPath: opts.RepoPath,
	})
	if err != nil {
		return nil, fmt.Errorf("open llm store: %w", err)
	}

	return &Store{
		stateStore: state.NewStore(stateDir),
		eventsDir:  eventsDir,
		llmStore:   llmStore,
		config:     cfg,
	}, nil
}

func defaultEventsDir() (string, error) {
	home, err := paths.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "incrementum", "agent", "events"), nil
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
		return s.llmStore.GetModel(explicit)
	}

	// Priority 2: Environment variable
	if envModel := os.Getenv("INCREMENTUM_AGENT_MODEL"); envModel != "" {
		return s.llmStore.GetModel(envModel)
	}

	// Priority 3: Per-task model
	if taskModel != "" {
		return s.llmStore.GetModel(taskModel)
	}

	// Priority 4: agent.model from config
	if s.config != nil && s.config.Agent.Model != "" {
		return s.llmStore.GetModel(s.config.Agent.Model)
	}

	// Priority 5: llm.model from config (final fallback)
	if defaultModel := s.llmStore.DefaultModel(); defaultModel != "" {
		return s.llmStore.GetModel(defaultModel)
	}

	return llm.Model{}, fmt.Errorf("no model configured: specify --model, set INCREMENTUM_AGENT_MODEL, or configure agent.model or llm.model in config")
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
	repoName, err := s.getRepoName(opts.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	// Generate session ID
	now := opts.StartedAt
	if now.IsZero() {
		now = time.Now()
	}
	sessionID := internalids.GenerateWithTimestamp(opts.RepoPath+opts.Prompt, now, internalids.DefaultLength)

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

	// Configure agent
	agentConfig := AgentConfig{
		Model:       model,
		Permissions: defaultBashPermissions(),
		WorkDir:     opts.WorkDir,
		Env:         opts.Env,
		SessionID:   sessionID,
		Version:     opts.Version,
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

	// Create event log file
	logPath := s.eventLogPath(sessionID)
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("create events dir: %w", err)
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("create event log: %w", err)
	}

	// Create wrapped event channel and result channel
	events := make(chan Event, 100)
	resultCh := make(chan RunResult, 1)

	handle := &RunHandle{
		Events:    events,
		sessionID: sessionID,
		handle:    internalHandle,
		resultCh:  resultCh,
	}

	// Start event forwarding goroutine
	go s.forwardEvents(ctx, internalHandle, events, resultCh, logFile, session)

	return handle, nil
}

// forwardEvents forwards events from internal agent to the public channel,
// logs them to disk, and updates session state on completion.
func (s *Store) forwardEvents(
	ctx context.Context,
	internalHandle *internalagent.RunHandle,
	events chan<- Event,
	resultCh chan<- RunResult,
	logFile *os.File,
	session Session,
) {
	defer close(events)
	defer close(resultCh)
	defer logFile.Close()

	writer := bufio.NewWriter(logFile)
	defer writer.Flush()

	eventIndex := 0

	// Forward all events
	for event := range internalHandle.Events {
		// Convert to SSE and log
		sse := EventToSSE(event)
		sse.ID = fmt.Sprintf("%d", eventIndex)
		eventIndex++

		// Write event to log as JSON line
		eventJSON, _ := json.Marshal(sse)
		writer.Write(eventJSON)
		writer.WriteString("\n")

		// Forward event
		select {
		case events <- event:
		case <-ctx.Done():
		}
	}

	// Flush remaining events
	writer.Flush()

	// Wait for final result
	internalResult, err := internalHandle.Wait()

	// Build result
	result := RunResult{
		SessionID: session.ID,
		ExitCode:  0,
		Messages:  internalResult.Messages,
		Usage:     internalResult.Usage,
	}

	// Update session based on result
	now := time.Now()
	session.UpdatedAt = now
	session.CompletedAt = now
	session.DurationSeconds = int(now.Sub(session.StartedAt).Seconds())
	session.TokensUsed = internalResult.Usage.Total
	session.Cost = internalResult.Usage.Cost.Total

	if err != nil || internalResult.Error != nil {
		session.Status = SessionFailed
		exitCode := 1
		session.ExitCode = &exitCode
		result.ExitCode = 1
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

	st, err := s.stateStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	var sessions []Session
	prefix := repoName + "/"
	for key, rawSession := range st.AgentSessions {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		session := sessionFromState(rawSession)
		sessions = append(sessions, session)
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

	st, err := s.stateStore.Load()
	if err != nil {
		return Session{}, fmt.Errorf("load state: %w", err)
	}

	// Try exact match first. The canonical state key is "<repoName>/<sessionID>".
	key := repoName + "/" + sessionID
	if rawSession, ok := st.AgentSessions[key]; ok {
		return sessionFromState(rawSession), nil
	}

	// Some call sites may pass the full state key as the session ID ("<repo>/<id>").
	// Accept that form as well.
	if rawSession, ok := st.AgentSessions[sessionID]; ok {
		return sessionFromState(rawSession), nil
	}

	// Try prefix match
	var matches []Session
	sessionIDLower := strings.ToLower(sessionID)
	prefix := repoName + "/"
	for stateKey, rawSession := range st.AgentSessions {
		if !strings.HasPrefix(stateKey, prefix) {
			continue
		}
		id := strings.TrimPrefix(stateKey, prefix)
		if strings.HasPrefix(strings.ToLower(id), sessionIDLower) {
			matches = append(matches, sessionFromState(rawSession))
		}
	}

	if len(matches) == 0 {
		return Session{}, fmt.Errorf("session not found: %s", sessionID)
	}
	if len(matches) > 1 {
		return Session{}, fmt.Errorf("ambiguous session ID: %s matches %d sessions", sessionID, len(matches))
	}

	return matches[0], nil
}

// Logs returns the raw event log for a session.
func (s *Store) Logs(repoPath, sessionID string) (string, error) {
	session, err := s.FindSession(repoPath, sessionID)
	if err != nil {
		return "", err
	}

	logPath := s.eventLogPath(session.ID)
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("event log not found for session %s", sessionID)
		}
		return "", fmt.Errorf("read event log: %w", err)
	}

	return string(data), nil
}

// Transcript returns a readable transcript of a session.
// Shows user and assistant messages without detailed tool call information.
func (s *Store) Transcript(repoPath, sessionID string) (string, error) {
	logContent, err := s.Logs(repoPath, sessionID)
	if err != nil {
		return "", err
	}

	return buildTranscript(logContent, false)
}

// TranscriptSnapshot returns a readable transcript by session ID.
// Unlike Transcript, this does not require the repoPath as the session ID
// uniquely identifies the event log file. This method includes tool output.
func (s *Store) TranscriptSnapshot(sessionID string) (string, error) {
	logPath := s.eventLogPath(sessionID)
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("event log not found for session %s", sessionID)
		}
		return "", fmt.Errorf("read event log: %w", err)
	}

	return buildTranscript(string(data), true)
}

// buildTranscript parses event log content and builds a readable transcript.
// If includeToolOutput is true, tool execution results are included.
func buildTranscript(logContent string, includeToolOutput bool) (string, error) {
	var builder strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(logContent))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var sse SSEEvent
		if err := json.Unmarshal([]byte(line), &sse); err != nil {
			continue
		}

		// Only include message events for transcript
		switch sse.Name {
		case "message.end":
			var event struct {
				Message struct {
					Role    string `json:"Role"`
					Content []struct {
						Type string `json:"Type"`
						Text string `json:"Text"`
					} `json:"Content"`
				} `json:"Message"`
			}
			if err := json.Unmarshal([]byte(sse.Data), &event); err != nil {
				continue
			}

			// Extract text content
			for _, block := range event.Message.Content {
				if block.Type == "text" && block.Text != "" {
					builder.WriteString("## Assistant\n\n")
					builder.WriteString(block.Text)
					builder.WriteString("\n\n")
				}
			}

		case "tool.end":
			if !includeToolOutput {
				continue
			}
			var event struct {
				Result struct {
					Content []struct {
						Type string `json:"Type"`
						Text string `json:"Text"`
					} `json:"Content"`
				} `json:"Result"`
			}
			if err := json.Unmarshal([]byte(sse.Data), &event); err != nil {
				continue
			}

			// Extract tool output text
			for _, block := range event.Result.Content {
				if block.Type == "text" && block.Text != "" {
					builder.WriteString("Tool output:\n")
					builder.WriteString(block.Text)
					builder.WriteString("\n\n")
				}
			}

		case "agent.start":
			// Note the session started
			builder.WriteString("# Agent Session\n\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan event log: %w", err)
	}

	return builder.String(), nil
}

// eventLogPath returns the path to the event log for a session.
func (s *Store) eventLogPath(sessionID string) string {
	return filepath.Join(s.eventsDir, sessionID+".jsonl")
}

// saveSession saves a session to state.
func (s *Store) saveSession(session Session) error {
	return s.stateStore.Update(func(st *state.State) error {
		if st.AgentSessions == nil {
			st.AgentSessions = make(map[string]state.AgentSession)
		}
		key := session.Repo + "/" + session.ID
		st.AgentSessions[key] = sessionToState(session)
		return nil
	})
}

// getRepoName gets or creates a repo name for the given path.
func (s *Store) getRepoName(repoPath string) (string, error) {
	return s.stateStore.GetOrCreateRepoName(repoPath)
}

// getRepoNameIfExists gets the repo name if it exists, or returns empty string.
func (s *Store) getRepoNameIfExists(repoPath string) (string, error) {
	st, err := s.stateStore.Load()
	if err != nil {
		return "", err
	}

	// Normalize paths for comparison to handle symlink differences (e.g., /var vs /private/var)
	normalizedRepoPath := paths.NormalizePath(repoPath)

	for name, info := range st.Repos {
		if paths.NormalizePath(info.SourcePath) == normalizedRepoPath {
			return name, nil
		}
	}

	return "", nil
}

// sessionToState converts a Session to state.AgentSession.
func sessionToState(session Session) state.AgentSession {
	return state.AgentSession{
		ID:              session.ID,
		Repo:            session.Repo,
		Status:          state.AgentSessionStatus(session.Status),
		Model:           session.Model,
		CreatedAt:       session.CreatedAt,
		StartedAt:       session.StartedAt,
		UpdatedAt:       session.UpdatedAt,
		CompletedAt:     session.CompletedAt,
		ExitCode:        session.ExitCode,
		DurationSeconds: session.DurationSeconds,
		TokensUsed:      session.TokensUsed,
		Cost:            session.Cost,
	}
}

// sessionFromState converts a state.AgentSession to Session.
func sessionFromState(stateSession state.AgentSession) Session {
	return Session{
		ID:              stateSession.ID,
		Repo:            stateSession.Repo,
		Status:          SessionStatus(stateSession.Status),
		Model:           stateSession.Model,
		CreatedAt:       stateSession.CreatedAt,
		StartedAt:       stateSession.StartedAt,
		UpdatedAt:       stateSession.UpdatedAt,
		CompletedAt:     stateSession.CompletedAt,
		ExitCode:        stateSession.ExitCode,
		DurationSeconds: stateSession.DurationSeconds,
		TokensUsed:      stateSession.TokensUsed,
		Cost:            stateSession.Cost,
	}
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
			{Pattern: "*", Allow: true},
		},
	}
}
