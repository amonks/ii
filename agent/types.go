// Package agent runs an autonomous agent loop that executes tasks using LLM-driven
// tool calls. It provides built-in tools (bash, read, write, edit) and integrates
// with the llm package for model interactions.
//
// This package wraps internal/agent to add session state persistence, event logging
// to disk, and model resolution from configuration.
package agent

import (
	"time"

	internalagent "monks.co/pkg/agent"
	"monks.co/ii/internal/validation"
	"monks.co/pkg/llm"
)

// Re-export types from internal/agent for convenience
type (
	// AgentConfig configures an agent run.
	AgentConfig = internalagent.AgentConfig

	// BashPermissions controls which bash commands are allowed.
	BashPermissions = internalagent.BashPermissions

	// BashRule defines a single permission rule for bash commands.
	BashRule = internalagent.BashRule
)

// SessionStatus represents the state of an agent session.
type SessionStatus string

const (
	// SessionActive indicates the session is currently running.
	SessionActive SessionStatus = "active"
	// SessionCompleted indicates the session completed successfully.
	SessionCompleted SessionStatus = "completed"
	// SessionFailed indicates the session failed.
	SessionFailed SessionStatus = "failed"
)

// ValidSessionStatuses returns all valid session status values.
func ValidSessionStatuses() []SessionStatus {
	return []SessionStatus{SessionActive, SessionCompleted, SessionFailed}
}

// IsValid returns true if the status is a known value.
func (s SessionStatus) IsValid() bool {
	return validation.IsValidValue(s, ValidSessionStatuses())
}

// Session represents an agent session stored in state.
type Session struct {
	// ID is the unique session identifier.
	ID string `json:"id"`

	// Repo is the repository slug this session belongs to.
	Repo string `json:"repo"`

	// Status is the current session status.
	Status SessionStatus `json:"status"`

	// Model is the model ID used for this session.
	Model string `json:"model"`

	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`

	// StartedAt is when the session started running.
	StartedAt time.Time `json:"started_at"`

	// UpdatedAt is when the session was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// CompletedAt is when the session completed (set when completed/failed).
	CompletedAt time.Time `json:"completed_at"`

	// ExitCode is the exit code (0 for success, non-zero for failure).
	ExitCode *int `json:"exit_code,omitempty"`

	// DurationSeconds is the session duration in seconds.
	DurationSeconds int `json:"duration_seconds,omitempty"`

	// TokensUsed is the total number of tokens consumed.
	TokensUsed int `json:"tokens_used,omitempty"`

	// Cost is the total cost in dollars.
	Cost float64 `json:"cost,omitempty"`
}

// RunOptions configures a new agent run.
type RunOptions struct {
	// RepoPath is the source repository path.
	RepoPath string

	// WorkDir is the working directory for tools.
	WorkDir string

	// Prompt contains structured content for the agent.
	Prompt internalagent.PromptContent

	// Model is the model ID to use. If empty, resolved via priority chain.
	Model string

	// StartedAt is when the run was initiated.
	StartedAt time.Time

	// Version is the version string (commit ID) to include in the User-Agent header.
	Version string

	// Env contains additional environment variables.
	Env []string

	// InputCh receives additional user input for interactive sessions.
	// If nil, the agent runs in single-shot mode.
	InputCh <-chan string
}

// RunHandle provides access to a running agent session.
type RunHandle struct {
	// sessionID is the session ID for this run.
	sessionID string

	// internal handle
	handle *internalagent.RunHandle

	// resultCh receives the final result.
	resultCh chan RunResult
}

// Wait blocks until the agent completes and returns the result.
func (h *RunHandle) Wait() (RunResult, error) {
	result := <-h.resultCh
	return result, nil
}

// RunResult contains the result of an agent run.
type RunResult struct {
	// SessionID is the unique identifier for this session.
	SessionID string

	// ExitCode is 0 on success, non-zero on failure.
	ExitCode int

	// Error contains the error message when ExitCode is non-zero.
	// This field is optional and may be empty even when ExitCode != 0;
	// not all failure conditions produce a detailed error message.
	Error string

	// Messages contains the full conversation history.
	Messages []llm.Message

	// Usage contains aggregate token usage and costs.
	Usage llm.Usage

	// ContextWindow is the model's context window size, for diagnostics.
	ContextWindow int
}
