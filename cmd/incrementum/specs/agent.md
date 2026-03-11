# Agent Package

## Overview

The agent package runs an autonomous agent loop that executes tasks using LLM-driven
tool calls. It provides built-in tools (bash, read, write, edit) and integrates with
the llm package for model interactions.

## Packages

### monks.co/pkg/agent (standalone module)

The core agent loop, formerly `internal/agent`, now a standalone module at `pkg/agent/`. Provides:

- Agent loop: prompt -> response -> tool execution -> repeat
- Built-in tools: bash, read, write, edit
- Typed event streaming
- Bash command permission filtering

### agent

A wrapper around `monks.co/pkg/agent` that adds:

- Session state persistence
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
   a. If `InputCh` is configured, emit `WaitingForInputEvent` and wait for user input.
      Whitespace-only lines are ignored (the agent keeps waiting for input).
   b. If the input channel closes, return the final message.
   c. Otherwise, append the new user message and continue.
```

### Termination Conditions

- Model returns response with no tool calls (natural completion)
  - If an input channel is configured, the agent waits for new user input and continues.
  - Whitespace-only input lines are ignored (the agent keeps waiting).
  - If the input channel is closed, the agent terminates normally.
- Model returns `StopReasonMaxTokens` (context overflow)
- Model returns `StopReasonError` (LLM error)
- Context cancelled (aborted)

### Error Handling

- **Tool errors**: Returned to the LLM as tool results with `IsError: true`, allowing
  the model to retry or adjust its approach. Tool errors do not terminate the agent.
- **Transient LLM API errors**: Automatically retried with exponential backoff using
  `llm.StreamWithRetry`. Retryable errors include rate limits (429), server errors
  (500, 502, 503, 504), and network failures.
- **Stream EOFs**: If a stream ends with an unexpected EOF, the agent retries the
  LLM request up to two additional times before treating it as a fatal stream
  error.
- **Non-transient errors**: Terminate the agent with an error result.

## Configuration

### AgentConfig

```go
type AgentConfig struct {
    Model       llm.Model
    Permissions BashPermissions
    WorkDir     string // Working directory for tools
    Env         []string // Extra environment variables for tool execution
    InputCh     <-chan string // Optional input channel for interactive sessions
    CacheRetention llm.CacheRetention // Prompt caching preference (default: short)
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

#### Compound Commands

For compound commands containing shell operators (`&&`, `||`, `;`, `|`), each
sub-command is checked independently. All sub-commands must be allowed for the
entire command to be allowed.

For example, with a rule `{Pattern: "rm *", Allow: false}`:
- `rm foo` → denied (matches `rm *`)
- `cd /tmp && rm foo` → denied (sub-command `rm foo` matches `rm *`)
- `echo worm juice` → allowed (no sub-command matches `rm *`)
- `ls | grep foo` → both `ls` and `grep foo` must be allowed

This is not a security boundary—it handles typical shell usage patterns without
attempting to be robust against intentionally crafted inputs designed to
circumvent the mechanism.

#### Example

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

### task

Spawns a subagent to handle a task. Use this for complex multi-step operations
that benefit from focused context. The subagent runs synchronously and returns
its final text response.

Parameters:
- `description` (string, required): A short (3-5 word) description of the task
- `prompt` (string, required): The task for the agent to perform
- `subagent_type` (string, required): The type of specialized agent to use

### task_with_context

Spawns a subagent to handle a task while supplying the full prompt context payload
(project workflow context, context file content, test commands, and workflow phase
content). Use this when the caller already has prompt context assembled and wants
subagents to inherit it without relying on repository fallback logic.

Parameters:
- `description` (string, required): A short (3-5 word) description of the task
- `prompt` (string, required): The task for the agent to perform
- `subagent_type` (string, required): The type of specialized agent to use
- `project_context` (string[], required): Workflow context, review questions, and review instructions
- `context_files` (string[], required): Context file contents to include
- `test_commands` (string[], required): Test commands for the project
- `phase_content` (string, required): Workflow phase content

Supported subagent types:
- `general`: General-purpose agent with full tool access (bash, read, write, edit) except task
- `explore`: Read-only agent for exploring codebases with bash and read tools
- `bash`: Command execution specialist with bash tool only

Subagents:
- Run synchronously and return their final text response
- Inherit the parent's model, permissions, and working directory
- Do not have access to the task tool (prevents recursive spawning)
- Do not emit events (their activity is internal to the parent agent)

The `toolExecutor.config` field controls task tool availability:
- Parent agents have `config` set, enabling subagent spawning
- Subagents have `config = nil`, preventing further spawning

`task_with_context` should be used when the caller already has prompt context
assembled and wants subagents to reuse it verbatim; `task` relies on repo fallback
logic to load workflow context and context files when its prompt content fields
are empty.

### Tool Validation Errors

When a tool is called with missing or invalid arguments, a detailed validation
error message is returned to the model showing:
- Which tool and parameter failed validation
- The expected type
- The actual arguments received (as JSON)

This allows the model to understand what went wrong and retry with correct parameters.

## System Prompt

The agent assembles a tiered system prompt with cache breakpoints to maximize
prompt reuse across phases.

Tiered structure:

- **Tier 1 (global, cache breakpoint)**: role description, task tool guidance,
  and editing guidelines.
- **Tier 2 (project, cache breakpoint)**: workflow context template, review
  questions template, default review instructions, context files (AGENTS.md/CLAUDE.md),
  and configured test commands.
- **Tier 3 (session, no breakpoint)**: working directory, current date (date-only),
  and phase-specific instructions.

Tool definitions (name, description, parameter schemas) are provided in the
request `tools` field instead of the system prompt.

The system prompt is built via `BuildSystemBlocks(workDir string, content PromptContent) []llm.SystemBlock`.
The working directory is included to help the model understand the context for
relative path resolution.

## AGENTS.md (context files)

Context files (`AGENTS.md` or `CLAUDE.md`) provide persistent, local instructions to the
agent without changing the global system prompt. The agent discovers and loads these files
following a specific order, returning their contents for inclusion in the tier-2 system
prompt block.

### Discovery Order

1. **Global config directory** (`~/.config/incrementum/`): If an `AGENTS.md` or `CLAUDE.md`
   file exists here, it is loaded first.

2. **Ancestor directories**: Starting from the filesystem root and walking down to the
   working directory, any `AGENTS.md` or `CLAUDE.md` files found are collected in order
   (root to working directory).

Within each directory, `AGENTS.md` takes precedence over `CLAUDE.md` (if both exist,
only `AGENTS.md` is used).

### Path Handling

- The working directory is resolved to an absolute, cleaned path before ancestor traversal.
  This ensures that relative paths (e.g., `.` or `./subdir`) are handled correctly.
- Returned `ContextFile.Path` values are canonicalized (absolute + cleaned).
- Deduplication uses canonicalized paths, so the same file won't be included twice even
  if accessed via different textual path representations (e.g., `/a/b/..` vs `/a`).
  Note: symlinks are not resolved, so different symlinks pointing to the same file may
  result in duplicate content.

### Concatenation

All discovered context files are concatenated in discovery order (global first, then
ancestors from root to working directory), separated by blank lines. The combined
content is appended to the tier-2 system prompt block.

### Example

Given this directory structure:
```
~/.config/incrementum/
  AGENTS.md           # "Global instructions"
/home/user/projects/
  AGENTS.md           # "Project-wide rules"
/home/user/projects/myapp/
  CLAUDE.md           # "App-specific context"
/home/user/projects/myapp/src/
  (working directory, no context file)
```

The tier-2 context file block would be:
```
Global instructions

Project-wide rules

App-specific context
```

If no context files are found, no tier-2 context file block is added.

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

// Waiting for interactive input
type WaitingForInputEvent struct {
    TurnIndex int
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
    Arguments  map[string]any
    Result     llm.ToolResultMessage
}
```

## Core Package API (monks.co/pkg/agent)

```go
type PromptContent struct {
    ProjectContext []string // rendered templates: workflow context, review questions, review instructions
    ContextFiles   []string // AGENTS.md/CLAUDE.md contents
    TestCommands   []string // from config
    PhaseContent   string   // phase-specific instructions
    UserContent    string   // todo block, series log, feedback, commit message
}

type RunHandle struct {}

func (h *RunHandle) Wait() (RunResult, error)

type RunResult struct {
    Messages []llm.Message  // Full conversation history
    Usage    llm.Usage      // Aggregate token usage and cost
    Error    error          // Non-nil if agent failed
}

func Run(ctx context.Context, prompt PromptContent, config AgentConfig) (*RunHandle, error)
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
}
```

### Run Options

```go
type RunOptions struct {
    RepoPath  string    // Source repository path (for config loading and session grouping)
    WorkDir   string    // Working directory for tool execution (may be a workspace within the repo)
    Prompt    PromptContent
    Model     string    // Model ID; resolved via priority chain
    StartedAt time.Time
    Version   string    // Version string (commit ID) included in User-Agent header
    Env       []string  // Additional environment variables passed to tool executions
    InputCh   <-chan string // Optional input channel for interactive sessions
}
```

Note: `RepoPath` and `WorkDir` may differ when running in a jj workspace. The `RepoPath`
identifies the source repository for configuration and session organization, while `WorkDir`
is the actual directory where the agent operates (e.g., a workspace directory). This allows
parallel jobs to work in separate workspace directories while sharing the same repository
configuration.

### Run Handle

```go
type RunHandle struct {}

func (h *RunHandle) Wait() (RunResult, error)

type RunResult struct {
    SessionID     string
    ExitCode      int
    Error         string  // Optional: error message when ExitCode is non-zero (best-effort)
    Messages      []llm.Message
    Usage         llm.Usage
    ContextWindow int     // Model's context window size, for diagnostics
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

Agent state is stored in SQLite via `internal/db` and the `agent` package's
store. Agent sessions live in the `agent_sessions` table and are keyed by repo
slug and session ID.

### Session Fields (DB columns)

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

- Session state is stored in the SQLite database under
  `~/.local/state/incrementum/state.db`.

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
2. Call `handle.Wait()` to get the final result
3. Use the session ID for logging and auditing

The job runner injects agent runs via a function parameter, allowing tests
to provide mock implementations.
