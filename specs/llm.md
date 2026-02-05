# LLM Package

## Overview

The llm package provides a unified abstraction for interacting with LLM providers.
It supports streaming completions, tool calling, prompt caching, and usage tracking
across Anthropic Messages, OpenAI Completions, and OpenAI Responses APIs.

## Packages

### internal/llm

The core LLM abstraction with no persistence. Provides:

- Unified message types across providers
- Streaming completions via channels
- Tool definitions using Go struct tags
- Prompt caching with TTL control
- Thinking/reasoning mode support
- Usage and cost tracking

### llm

A wrapper around `internal/llm` that adds:

- Completion history storage to disk
- CLI support for `ii llm` subcommands
- Model listing from configuration

## Message Types

### UserMessage

Represents user input to the model.

```go
type UserMessage struct {
    Role      string         // "user"
    Content   []ContentBlock // Text and/or images
    Timestamp time.Time
}
```

### AssistantMessage

Represents model output.

```go
type AssistantMessage struct {
    Role         string          // "assistant"
    Content      []ContentBlock  // Text, thinking, and/or tool calls
    Model        string          // Model ID used
    API          API             // Which API was used
    Provider     string          // Provider name
    Usage        Usage           // Token counts and costs
    StopReason   StopReason      // Why generation stopped
    ErrorMessage string          // Set when StopReason is "error"
    Timestamp    time.Time
}
```

### ToolResultMessage

Represents tool execution results returned to the model.

```go
type ToolResultMessage struct {
    Role       string         // "toolResult"
    ToolCallID string         // Matches the ToolCall.ID
    ToolName   string         // Tool that was called
    Content    []ContentBlock // Text and/or images
    IsError    bool           // Whether the tool execution failed
    Timestamp  time.Time
}
```

## Content Blocks

```go
type ContentBlock interface {
    contentBlock()
}

type TextContent struct {
    Type string // "text"
    Text string
}

type ThinkingContent struct {
    Type     string // "thinking"
    Thinking string
}

type ImageContent struct {
    Type     string // "image"
    Data     string // base64 encoded
    MimeType string
}

type ToolCall struct {
    Type      string         // "toolCall"
    ID        string         // Unique identifier
    Name      string         // Tool name
    Arguments map[string]any // Parsed arguments
}
```

## API Types

```go
type API string

const (
    APIAnthropicMessages  API = "anthropic-messages"
    APIOpenAICompletions  API = "openai-completions"
    APIOpenAIResponses    API = "openai-responses"
)
```

## Stop Reasons

```go
type StopReason string

const (
    StopReasonEnd      StopReason = "end"       // Natural completion
    StopReasonToolUse  StopReason = "tool_use"  // Model wants to call tools
    StopReasonMaxTokens StopReason = "max_tokens"
    StopReasonError    StopReason = "error"
    StopReasonAborted  StopReason = "aborted"   // Cancelled via context
)
```

## Model Configuration

Models are defined in the incrementum configuration file. The package includes
built-in knowledge of well-known models (context windows, pricing, capabilities)
that users can reference.

### Configuration Structure

```toml
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "op read op://Private/Anthropic/credential"
models = ["claude-sonnet-4-20250514", "claude-haiku-4-20250514"]

[[llm.providers]]
name = "internal-claude"
api = "anthropic-messages"
base-url = "https://internal-claude.example.com"
# no api-key-command means no auth required
models = ["claude-sonnet-4-20250514"]
```

### Model Type

```go
type Model struct {
    ID                     string   // e.g., "claude-sonnet-4-20250514"
    Name                   string   // Human-readable name
    API                    API      // Which API style to use
    Provider               string   // Provider name from config
    BaseURL                string   // API endpoint
    APIKey                 string   // API key (resolved from api-key-command)
    ContextWindow          int      // Max context tokens
    MaxTokens              int      // Max output tokens
    Reasoning              bool     // Supports thinking mode
    UseMaxCompletionTokens bool     // Use max_completion_tokens instead of max_tokens (for o1/o3/o4 models)
    InputTypes             []string // "text", "image"
    Cost                   Cost     // Pricing per million tokens
}

type Cost struct {
    Input      float64 // $/million input tokens
    Output     float64 // $/million output tokens
    CacheRead  float64 // $/million cached input tokens
    CacheWrite float64 // $/million cache write tokens
}
```

### API Key Resolution

API keys are retrieved by executing the command specified in `api-key-command`.
The command's stdout is trimmed and used as the key. Results are cached for
the process lifetime.

### HTTP User-Agent

All LLM HTTP requests MUST set a `User-Agent` header.

- Normal runtime value:

  `incrementum [$version] $dirname`

  where `$version` is the `commit_id` printed by `ii -v` and `$dirname` is the
  base name of the repository root path.

- During `go test` (`testing.Testing() == true`):

  `incrementum TEST`

This is implemented in `internal/llm.UserAgent(repoPath, version)` and is passed
via `internal/llm.StreamOptions.UserAgent`.


Tools are defined using Go structs with JSON tags. The package uses reflection
to generate JSON Schema from struct definitions.

```go
type CalculatorParams struct {
    A         float64 `json:"a" jsonschema:"description=First number"`
    B         float64 `json:"b" jsonschema:"description=Second number"`
    Operation string  `json:"operation" jsonschema:"enum=add,enum=subtract,enum=multiply,enum=divide"`
}

type Tool struct {
    Name        string
    Description string
    Parameters  any // Struct type for JSON Schema generation
}
```

## Streaming

### Stream Events

```go
type StreamEvent interface {
    streamEvent()
}

type StartEvent struct {
    Partial AssistantMessage
}

type TextDeltaEvent struct {
    ContentIndex int
    Delta        string
    Partial      AssistantMessage
}

type ThinkingDeltaEvent struct {
    ContentIndex int
    Delta        string
    Partial      AssistantMessage
}

type ToolCallDeltaEvent struct {
    ContentIndex int
    Delta        string // JSON fragment
    Partial      AssistantMessage
}

type ToolCallEndEvent struct {
    ContentIndex int
    ToolCall     ToolCall
    Partial      AssistantMessage
}

type DoneEvent struct {
    Reason  StopReason
    Message AssistantMessage
}

type ErrorEvent struct {
    Reason  StopReason
    Message AssistantMessage
}
```

### Streaming API

```go
type StreamHandle struct {
    Events <-chan StreamEvent
}

func (h *StreamHandle) Wait() (AssistantMessage, error)

func Stream(ctx context.Context, model Model, req Request, opts StreamOptions) (*StreamHandle, error)
```

### Request Type

```go
type Request struct {
    SystemPrompt string
    Messages     []Message // UserMessage, AssistantMessage, or ToolResultMessage
    Tools        []Tool
}
```

### Stream Options

```go
type StreamOptions struct {
    Temperature    *float64
    MaxTokens      *int
    CacheRetention CacheRetention // "none", "short", "long"
    ThinkingLevel  ThinkingLevel  // "off", "minimal", "low", "medium", "high", "xhigh"
}

type CacheRetention string

const (
    CacheNone  CacheRetention = "none"
    CacheShort CacheRetention = "short" // ~5 minutes
    CacheLong  CacheRetention = "long"  // ~1 hour
)

type ThinkingLevel string

const (
    ThinkingOff     ThinkingLevel = "off"
    ThinkingMinimal ThinkingLevel = "minimal"
    ThinkingLow     ThinkingLevel = "low"
    ThinkingMedium  ThinkingLevel = "medium"
    ThinkingHigh    ThinkingLevel = "high"
    ThinkingXHigh   ThinkingLevel = "xhigh"
)
```

## Usage Tracking

```go
type Usage struct {
    Input      int  // Input tokens
    Output     int  // Output tokens
    CacheRead  int  // Tokens read from cache
    CacheWrite int  // Tokens written to cache
    Total      int  // Total tokens
    Cost       UsageCost
}

type UsageCost struct {
    Input      float64
    Output     float64
    CacheRead  float64
    CacheWrite float64
    Total      float64
}
```

## Error Handling

- Network errors and rate limits trigger automatic retry with exponential backoff
- Default retry configuration: up to 5 retries (6 total attempts), starting at 1 second
  and doubling up to a maximum of 30 seconds between attempts
- Retryable errors: HTTP 429 (rate limit), 500, 502, 503, 504, and network failures
- Non-retryable errors: HTTP 4xx (except 429) return immediately without retry
- Retries are capped at a configurable maximum delay
- Context cancellation stops retries and returns `StopReasonAborted`
- Unrecoverable errors return `StopReasonError` with `ErrorMessage` populated

## Provider Implementations

Each provider implements streaming by:

1. Creating the appropriate SDK client with model's base URL and API key
2. Converting unified `Request` to provider-specific format
3. Making streaming API call
4. Converting provider events to unified `StreamEvent` format
5. Building final `AssistantMessage` with usage statistics

### Message Transformation

When switching models between turns, previous thinking blocks are converted
to text content. Tool call IDs are normalized to meet provider constraints.

## Public Package (llm/)

### Store

```go
type Store struct {
    // internal state
}

func Open() (*Store, error)
func OpenWithOptions(opts Options) (*Store, error)

type Options struct {
    StateDir    string // Default: ~/.local/state/incrementum
    HistoryDir  string // Default: ~/.local/share/incrementum/llm/history
    RepoPath    string // If set, loads project-specific config; otherwise global only
}
```

### Well-Known Models

The package includes built-in knowledge of well-known models. When a model ID
matches a known model, its capabilities and pricing are automatically populated:

- Claude models: claude-sonnet-4-20250514, claude-haiku-4-20250514, claude-3-5-sonnet-20241022, etc.
- GPT-4o models: gpt-4o, gpt-4o-mini, etc.
- Reasoning models: o1, o1-mini, o3-mini

Unknown models receive conservative defaults (128k context, 4k max tokens, text-only).

### Completion History

```go
type Completion struct {
    ID          string
    Model       string
    Request     Request
    Response    AssistantMessage
    CreatedAt   time.Time
}

func (s *Store) ListCompletions() ([]Completion, error)
func (s *Store) GetCompletion(id string) (Completion, error)
```

### Model Access

```go
func (s *Store) ListModels() ([]Model, error)
func (s *Store) GetModel(id string) (Model, error)
```

`GetModel` supports prefix matching - "claude-sonnet" will match "claude-sonnet-4-20250514"
if it's the only match. Returns `ErrAmbiguousModel` if multiple models match the prefix.

### Streaming with History

```go
func (s *Store) Stream(ctx context.Context, model Model, req Request, opts StreamOptions) (*StreamHandle, error)
```

This wraps `internal/llm.Stream` and records completions to history.

## Commands

### `ii llm complete [prompt]`

- Prompt is read from stdin when no argument is provided
- `--model` selects the model; defaults to `llm.model` from config if not specified
- `--temperature` sets sampling temperature
- `--max-tokens` sets output limit
- Streams response to stdout
- Records completion to history

### `ii llm models [--json]`

- Lists configured models
- Default output is a table: `MODEL`, `PROVIDER`, `API`
- `--json` outputs JSON array

### `ii llm list [--json]`

- Lists historical completions
- Default output is a table: `ID`, `MODEL`, `AGE`
- `--json` outputs JSON array

### `ii llm show <completion-id>`

- Resolves completion by ID prefix (case-insensitive, unambiguous)
- Prints the completion request and response
