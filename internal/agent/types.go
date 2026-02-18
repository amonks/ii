// Package agent provides the core agent loop without persistence.
//
// This package implements the agent loop (prompt -> LLM -> tool execution -> repeat),
// built-in tools (bash, read, write, edit), bash command permission filtering,
// and typed event streaming.
package agent

import (
	"github.com/amonks/incrementum/internal/llm"
)

// AgentConfig configures an agent run.
type AgentConfig struct {
	// Model is the LLM model to use for completions.
	Model llm.Model

	// Permissions controls which bash commands are allowed.
	Permissions BashPermissions

	// WorkDir is the working directory for tools.
	// If empty, the current working directory is used.
	WorkDir string

	// GlobalConfigDir is the global config directory (e.g., ~/.config/incrementum)
	// for loading global AGENTS.md or CLAUDE.md files.
	// If empty, no global context is loaded.
	GlobalConfigDir string

	// Env contains additional environment variables for tool execution.
	Env []string

	// SessionID is an optional identifier for grouping related API requests
	// into sessions for observability purposes.
	SessionID string

	// Version is the version string (commit ID) to include in the User-Agent header.
	Version string

	// InputCh receives additional user input for interactive sessions.
	// If nil, the agent runs in single-shot mode.
	InputCh <-chan string

	// CacheRetention controls prompt caching for providers that support it.
	// Defaults to CacheShort when unset.
	CacheRetention llm.CacheRetention
}

// BashPermissions controls which bash commands are allowed.
// Rules are evaluated in order; first match wins. Default is deny.
type BashPermissions struct {
	Rules []BashRule
}

// BashRule defines a single permission rule for bash commands.
type BashRule struct {
	// Pattern is a glob pattern matching the command.
	Pattern string

	// Allow indicates whether matching commands are allowed (true) or denied (false).
	Allow bool
}

// RunResult contains the result of an agent run.
type RunResult struct {
	// Messages contains the full conversation history.
	Messages []llm.Message

	// Usage contains aggregate token usage and costs across all LLM calls.
	Usage llm.Usage

	// Error is non-nil if the agent failed.
	Error error
}

// RunHandle provides access to a running agent.
type RunHandle struct {
	// Events receives typed events from the agent run.
	// The channel is closed when the agent completes.
	Events <-chan Event

	result chan RunResult
}

// Wait blocks until the agent completes and returns the result.
func (h *RunHandle) Wait() (RunResult, error) {
	// Drain events first
	for range h.Events {
	}

	result := <-h.result
	return result, result.Error
}

// Event is an interface implemented by all agent event types.
type Event interface {
	agentEvent()
}

// AgentStartEvent indicates the agent run has started.
type AgentStartEvent struct {
	Config AgentConfig
}

func (AgentStartEvent) agentEvent() {}

// AgentEndEvent indicates the agent run has completed.
type AgentEndEvent struct {
	// Messages contains the full conversation history.
	Messages []llm.Message

	// Usage contains aggregate token usage and costs.
	Usage llm.Usage
}

func (AgentEndEvent) agentEvent() {}

// TurnStartEvent indicates a new turn (LLM call + tool executions) has started.
type TurnStartEvent struct {
	TurnIndex int
}

func (TurnStartEvent) agentEvent() {}

// TurnEndEvent indicates a turn has completed.
type TurnEndEvent struct {
	TurnIndex   int
	Message     llm.AssistantMessage
	ToolResults []llm.ToolResultMessage
}

func (TurnEndEvent) agentEvent() {}

// MessageStartEvent indicates the LLM has started streaming a response.
type MessageStartEvent struct {
	TurnIndex int
	Partial   llm.AssistantMessage
}

func (MessageStartEvent) agentEvent() {}

// MessageUpdateEvent contains a streaming delta from the LLM.
type MessageUpdateEvent struct {
	TurnIndex   int
	StreamEvent llm.StreamEvent
	Partial     llm.AssistantMessage
}

func (MessageUpdateEvent) agentEvent() {}

// MessageEndEvent indicates the LLM has finished streaming a response.
type MessageEndEvent struct {
	TurnIndex int
	Message   llm.AssistantMessage
}

func (MessageEndEvent) agentEvent() {}

// ToolExecutionStartEvent indicates a tool execution has started.
type ToolExecutionStartEvent struct {
	TurnIndex  int
	ToolCallID string
	ToolName   string
	Arguments  map[string]any
}

func (ToolExecutionStartEvent) agentEvent() {}

// ToolExecutionEndEvent indicates a tool execution has completed.
type ToolExecutionEndEvent struct {
	TurnIndex  int
	ToolCallID string
	ToolName   string
	Arguments  map[string]any
	Result     llm.ToolResultMessage
}

func (ToolExecutionEndEvent) agentEvent() {}

// WaitingForInputEvent indicates the agent is waiting for additional user input.
type WaitingForInputEvent struct {
	TurnIndex int
}

func (WaitingForInputEvent) agentEvent() {}

// SSEEvent represents an event in SSE format.
type SSEEvent struct {
	ID   string
	Name string
	Data string // JSON encoded
}
