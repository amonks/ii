# Internal LLM

## Overview

The `internal/llm` package provides the core LLM abstraction without persistence.
See [llm.md](./llm.md) for the full specification.

## Scope

This package implements:

- Message types (UserMessage, AssistantMessage, ToolResultMessage)
- Content blocks (TextContent, ThinkingContent, ImageContent, ToolCall)
- Streaming via channels with typed events
- Tool schema generation from Go structs
- Provider implementations (Anthropic, OpenAI Completions, OpenAI Responses)
- Usage tracking with cost calculation
- Retry with exponential backoff

This package does NOT handle:

- Completion history storage
- Model configuration loading (receives Model structs from caller)
- CLI commands

## API

```go
func Stream(ctx context.Context, model Model, req Request, opts StreamOptions) (*StreamHandle, error)
func StreamWithRetry(ctx context.Context, model Model, req Request, opts StreamOptions, config RetryConfig) (*StreamHandle, error)
func GenerateSchema(v any) *Schema
```

See [llm.md](./llm.md) for type definitions.
