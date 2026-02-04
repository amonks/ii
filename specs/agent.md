# Agent Package

## Overview

The agent package runs an autonomous agent loop that executes tasks using LLM-driven
tool calls. It provides built-in tools (bash, read, write, edit) and integrates with
the llm package for model interactions.

## Packages

### internal/agent

The core agent loop with no persistence. Provides:

- Agent loop: prompt -> response -> tool execution -> repeat
- Built-in tools: bash, read, write, edit
- Typed event streaming
- Bash command permission filtering

### agent

A wrapper around `internal/agent` that adds:

- Session state persistence
- Event logging to disk
- CLI support for `ii agent` subcommands

## Agent Loop

### Flow

```
1. Send user prompt + system prompt + tools to LLM
2. Stream assistant response
3. If response contains tool calls:
   a. Execute each tool
   b. Collect tool results
   c. Go to step 1 with tool results appended
4. If no tool calls (natural end):
   a. Return final message
```

### Termination Conditions

- Model returns response with no tool calls (natural completion)
- Model returns `StopReasonMaxTokens` (context overflow)
- Model returns `StopReasonError` (LLM error)
- Context cancelled (aborted)

### Error Handling

- **Tool errors**: Returned to the LLM as tool results with `IsError: true`, allowing
  the model to retry or adjust its approach. Tool errors do not terminate the agent.
- **Transient LLM API errors**: Automatically retried with exponential backoff using
  `llm.StreamWithRetry`. Retryable errors include rate limits (429), server errors
  (500, 502, 503, 504), and network failures.
- **Non-transient errors**: Terminate the agent with an error result.

## Configuration

### AgentConfig

```go
type AgentConfig struct {
    Model       llm.Model
    Permissions BashPermissions
    WorkDir     string // Working directory for tools
    Env         []string // Extra environment variables for tool execution
}
```

### Bash Permissions

Bash command permissions use glob patterns with allow/deny rules.
Rules are evaluated in order; first match wins. Default is deny.

```go
type BashPermissions struct {
    Rules []BashRule
}

type BashRule struct {
    Pattern string // Glob pattern matching command
    Allow   bool
}
```

Example matching typical bash permission rules:

```go
BashPermissions{
    Rules: []BashRule{
        {Pattern: "jj diff", Allow: true},
        {Pattern: "jj diff *", Allow: true},
        {Pattern: "jj file", Allow: true},
        {Pattern: "jj file *", Allow: true},
        {Pattern: "jj log", Allow: true},
        {Pattern: "jj log *", Allow: true},
        {Pattern: "jj show", Allow: true},
        {Pattern: "jj show *", Allow: true},
        {Pattern: "jj *", Allow: false},
        {Pattern: "*", Allow: true},
    },
}
```

## Built-in Tools

### bash

Executes shell commands in the working directory.

Parameters:
- `command` (string, required): The command to execute
- `timeout` (int, optional): Timeout in seconds, default 120

Returns stdout/stderr. Command is checked against bash permissions before
execution; denied commands return an error result. Commands inherit the current
process environment plus any `AgentConfig.Env` entries.

### read

Reads file contents.

Parameters:
- `path` (string, required): Path to file (absolute or relative to working directory)
- `offset` (int, optional): Line offset (0-based)
- `limit` (int, optional): Number of lines, default 2000

Returns file content with line numbers. Handles binary files, missing files,
and permission errors gracefully.

### write

Writes content to a file.

Parameters:
- `path` (string, required): Path to file (absolute or relative to working directory)
- `content` (string, required): Content to write

Creates parent directories as needed. Returns success/error status.

### edit

Performs text replacement in a file.

Parameters:
- `path` (string, required): Path to file (absolute or relative to working directory)
- `old_string` (string, required): Text to find
- `new_string` (string, required): Replacement text
- `replace_all` (bool, optional): Replace all occurrences, default false

Returns error if `old_string` not found or found multiple times (when
`replace_all` is false).

### Tool Validation Errors

When a tool is called with missing or invalid arguments, a detailed validation
error message is returned to the model showing:
- Which tool and parameter failed validation
- The expected type
- The actual arguments received (as JSON)

This allows the model to understand what went wrong and retry with correct parameters.

## System Prompt

The agent uses a dynamically generated system prompt that includes:

- Current working directory
- Current date and time
- Available tools and their parameter schemas
- Code editing best practices (read before edit, use precise edits, prefer edit over write)
- Guidelines for handling tool errors gracefully

The system prompt is built via `BuildSystemPrompt(workDir string)` and is not externally
configurable. The working directory is included to help the model understand the context
for relative path resolution.

## AGENTS.md (agent prelude)

If an `AGENTS.md` file exists in the agent working directory, its contents are prepended
to the *first user message* in the session (followed by a blank line). This allows repos
to provide persistent, local instructions without changing the global system prompt.

If `AGENTS.md` is missing or empty, nothing is added.

## Event Streaming

### Typed Events

```go
type Event interface {
    agentEvent()
}

// Lifecycle events
type AgentStartEvent struct {
    Config AgentConfig
}

type AgentEndEvent struct {
    Messages []llm.Message
    Usage    llm.Usage // Aggregate usage
}

// Turn events (one turn = one LLM call + tool executions)
type TurnStartEvent struct {
    TurnIndex int
}

type TurnEndEvent struct {
    TurnIndex   int
    Message     llm.AssistantMessage
    ToolResults []llm.ToolResultMessage
}

// Message streaming (wraps llm.StreamEvent)
type MessageStartEvent struct {
    TurnIndex int
    Partial   llm.AssistantMessage
}

type MessageUpdateEvent struct {
    TurnIndex   int
    StreamEvent llm.StreamEvent
    Partial     llm.AssistantMessage
}

type MessageEndEvent struct {
    TurnIndex int
    Message   llm.AssistantMessage
}

// Tool execution
type ToolExecutionStartEvent struct {
    TurnIndex  int
    ToolCallID string
    ToolName   string
    Arguments  map[string]any
}

type ToolExecutionEndEvent struct {
    TurnIndex  int
    ToolCallID string
    ToolName   string
    Result     llm.ToolResultMessage
}
```

### SSE Adapter

For compatibility with existing event logging, an adapter converts typed
events to SSE format (ID, Name, Data strings).

```go
func EventToSSE(event Event) SSEEvent

type SSEEvent struct {
    ID   string
    Name string
    Data string // JSON encoded
}
```

Note: API keys are automatically redacted when events are serialized to SSE
format to prevent sensitive credentials from appearing in event logs.

## Internal Package API (internal/agent)

```go
type RunHandle struct {
    Events <-chan Event
}

func (h *RunHandle) Wait() (RunResult, error)

type RunResult struct {
    Messages []llm.Message  // Full conversation history
    Usage    llm.Usage      // Aggregate token usage and cost
    Error    error          // Non-nil if agent failed
}

func Run(ctx context.Context, prompt string, config AgentConfig) (*RunHandle, error)
```

## Public Package API (agent/)

### Store

```go
type Store struct {
    // internal state
}

func Open() (*Store, error)
func OpenWithOptions(opts Options) (*Store, error)

type Options struct {
    StateDir  string // Default: ~/.local/state/incrementum
    EventsDir string // Default: ~/.local/share/incrementum/agent/events
}
```

### Run Options

```go
type RunOptions struct {
    RepoPath  string
    WorkDir   string
    Prompt    string
    Model     string    // Model ID; resolved via priority chain
    StartedAt time.Time
    Version   string    // Version string (commit ID) included in User-Agent header
    Env       []string  // Additional environment variables passed to tool executions
}
```

### Run Handle

```go
type RunHandle struct {
    Events <-chan Event
}

func (h *RunHandle) Wait() (RunResult, error)

type RunResult struct {
    SessionID string
    ExitCode  int
    Messages  []llm.Message
    Usage     llm.Usage
}

func (s *Store) Run(ctx context.Context, opts RunOptions) (*RunHandle, error)
```

### Session Management

```go
type Session struct {
    ID              string
    Repo            string
    Status          SessionStatus // "active", "completed", "failed"
    Model           string
    CreatedAt       time.Time
    StartedAt       time.Time
    UpdatedAt       time.Time
    CompletedAt     time.Time
    ExitCode        *int
    DurationSeconds int
    TokensUsed      int
    Cost            float64
}

type SessionStatus string

const (
    SessionActive    SessionStatus = "active"
    SessionCompleted SessionStatus = "completed"
    SessionFailed    SessionStatus = "failed"
)

func (s *Store) ListSessions(repoPath string) ([]Session, error)
func (s *Store) FindSession(repoPath, sessionID string) (Session, error)
```

### Log Access

```go
// Logs returns the event log for a session
func (s *Store) Logs(repoPath, sessionID string) (string, error)

// Transcript returns a readable transcript without tool output (prose only)
func (s *Store) Transcript(repoPath, sessionID string) (string, error)

// TranscriptSnapshot returns a readable transcript by session ID only.
// Unlike Transcript, this includes tool output.
func (s *Store) TranscriptSnapshot(sessionID string) (string, error)
```

### Model Resolution

Model selection follows this priority chain:

1. Explicit model in `RunOptions.Model`
2. `INCREMENTUM_AGENT_MODEL` environment variable
3. Per-task model from todo/job configuration
4. `job.implementation-model` from merged config (fallback)
5. Error if no model resolved

```go
func (s *Store) ResolveModel(explicit string, taskModel string) (llm.Model, error)
```

## State Model

Agent state is stored in the shared state file alongside workspace and
job state. It adds one top-level collection:

- `agent_sessions`: map of `repo-slug/session-id` to session info

### Session Fields (JSON keys)

- `id`: session id (e.g., `<generated>`)
- `repo`: repo slug
- `status`: `active`, `completed`, or `failed`
- `model`: model ID used
- `created_at`: timestamp
- `started_at`: timestamp
- `updated_at`: timestamp
- `completed_at`: timestamp (set when completed/failed)
- `exit_code`: integer exit code
- `duration_seconds`: duration in seconds
- `tokens_used`: total tokens consumed
- `cost`: total cost in dollars

## Storage

- Session state is stored in `~/.local/state/incrementum/state.json`
- Event logs are stored under `~/.local/share/incrementum/agent/events/<session-id>.jsonl`
- Events are written as newline-delimited JSON (one SSE event per line)

## Commands

### `ii agent run [prompt]`

- Prompt is read from stdin when no argument is provided
- `--model` selects the model; falls back to config/env priority chain
- Streams agent events to stderr (progress, tool calls)
- Final response written to stdout
- Creates session record in state
- Updates status, exit code, usage when complete
- Returns exit code 0 on success, non-zero on failure

### `ii agent list [--json] [--all]`

- Lists agent sessions for the current repo
- Default output is a table: `SESSION`, `STATUS`, `MODEL`, `AGE`, `DURATION`, `TOKENS`, `COST`
- Default shows only active sessions unless `--all` is provided
- Session ID column highlights shortest unique prefix

### `ii agent logs <session-id> [--json]`

- Resolves session by ID prefix (case-insensitive, unambiguous)
- Default output renders a readable transcript (user/assistant messages)
- With `--json`, prints the stored event log as JSONL to stdout

### `ii agent tail <session-id> [--json]`

- Resolves session by ID prefix (case-insensitive, unambiguous)
- Default (no `--json`): remains alive until the agent session ends
  - Polls the transcript snapshot
  - Prints only newly appended transcript content (append-only diff)
  - If the current transcript snapshot is not prefixed by the previous snapshot, prints the full snapshot (non-append fallback)
- With `--json`: remains alive until the agent session ends
  - Polls the stored event log (`events/<session-id>.jsonl`)
  - Writes only newly appended **complete** JSONL lines to stdout (append-only diff; requires a trailing `\n`)
  - Incomplete trailing JSON fragments are withheld until completed by a later poll
  - If the current JSONL snapshot is not prefixed by the previous snapshot, prints only **complete** lines (up to the last `\n`) (non-append fallback)

## Error Handling

- LLM errors trigger retry with exponential backoff (handled by llm package)
- Tool execution errors are returned to the model as error results
- Permission denied errors for bash commands are returned to the model
- Context cancellation stops the loop and marks session as failed
- Unrecoverable errors mark session as failed with exit code 1

## Integration with Job System

The job package uses the agent store:

1. Call `store.Run(RunOptions)` to start an agent session
2. Consume events from the handle for progress tracking
3. Call `handle.Wait()` to get the final result
4. Use the session ID for logging and auditing

The job runner injects agent runs via a function parameter, allowing tests
to provide mock implementations.
