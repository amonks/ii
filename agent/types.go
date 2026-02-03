// Package agent runs an autonomous agent loop that executes tasks using LLM-driven
// tool calls. It provides built-in tools (bash, read, write, edit) and integrates
// with the llm package for model interactions.
//
// This package wraps internal/agent to add session state persistence, event logging
// to disk, and model resolution from configuration.
package agent

import (
	"time"

	internalagent "github.com/amonks/incrementum/internal/agent"
	"github.com/amonks/incrementum/internal/validation"
	"github.com/amonks/incrementum/llm"
)

// Re-export types from internal/agent for convenience
type (
	// AgentConfig configures an agent run.
	AgentConfig = internalagent.AgentConfig

	// BashPermissions controls which bash commands are allowed.
	BashPermissions = internalagent.BashPermissions

	// BashRule defines a single permission rule for bash commands.
	BashRule = internalagent.BashRule

	// Event is an interface implemented by all agent event types.
	Event = internalagent.Event

	// AgentStartEvent indicates the agent run has started.
	AgentStartEvent = internalagent.AgentStartEvent

	// AgentEndEvent indicates the agent run has completed.
	AgentEndEvent = internalagent.AgentEndEvent

	// TurnStartEvent indicates a new turn has started.
	TurnStartEvent = internalagent.TurnStartEvent

	// TurnEndEvent indicates a turn has completed.
	TurnEndEvent = internalagent.TurnEndEvent

	// MessageStartEvent indicates the LLM has started streaming.
	MessageStartEvent = internalagent.MessageStartEvent

	// MessageUpdateEvent contains a streaming delta from the LLM.
	MessageUpdateEvent = internalagent.MessageUpdateEvent

	// MessageEndEvent indicates the LLM has finished streaming.
	MessageEndEvent = internalagent.MessageEndEvent

	// ToolExecutionStartEvent indicates a tool execution has started.
	ToolExecutionStartEvent = internalagent.ToolExecutionStartEvent

	// ToolExecutionEndEvent indicates a tool execution has completed.
	ToolExecutionEndEvent = internalagent.ToolExecutionEndEvent

	// SSEEvent represents an event in SSE format.
	SSEEvent = internalagent.SSEEvent
)

// Re-export functions from internal/agent
var EventToSSE = internalagent.EventToSSE

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
	// ID is the unique session identifier (e.g., "agt_...").
	ID string `json:"id"`

	// Repo is the repository slug this session belongs to.
	Repo string `json:"repo"`

	// Status is the current session status.
	Status SessionStatus `json:"status"`

	// Model is the model ID used for this session.
	Model string `json:"model"`

	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at,omitempty"`

	// StartedAt is when the session started running.
	StartedAt time.Time `json:"started_at"`

	// UpdatedAt is when the session was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// CompletedAt is when the session completed (set when completed/failed).
	CompletedAt time.Time `json:"completed_at,omitempty"`

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

	// Prompt is the user prompt to send to the agent.
	Prompt string

	// Model is the model ID to use. If empty, resolved via priority chain.
	Model string

	// StartedAt is when the run was initiated.
	StartedAt time.Time

	// Env contains additional environment variables.
	Env []string
}

// RunHandle provides access to a running agent session.
type RunHandle struct {
	// Events receives typed events from the agent run.
	// The channel is closed when the agent completes.
	Events <-chan Event

	// sessionID is the session ID for this run.
	sessionID string

	// internal handle
	handle *internalagent.RunHandle

	// resultCh receives the final result.
	resultCh chan RunResult
}

// Wait blocks until the agent completes and returns the result.
func (h *RunHandle) Wait() (RunResult, error) {
	// Drain events first
	for range h.Events {
	}

	result := <-h.resultCh
	return result, nil
}

// RunResult contains the result of an agent run.
type RunResult struct {
	// SessionID is the unique identifier for this session.
	SessionID string

	// ExitCode is 0 on success, non-zero on failure.
	ExitCode int

	// Messages contains the full conversation history.
	Messages []llm.Message

	// Usage contains aggregate token usage and costs.
	Usage llm.Usage
}
