# Internal Agent

## Overview

The `internal/agent` package provides the core agent loop without persistence.
See [agent.md](./agent.md) for the full specification.

## Scope

This package implements:

- Agent loop (prompt -> LLM -> tool execution -> repeat)
- Built-in tools (bash, read, write, edit)
- Bash command permission filtering
- Typed event streaming
- System prompt

This package does NOT handle:

- Session state persistence
- Event logging to disk
- CLI commands
- Model resolution (receives Model from caller)

## API

```go
func Run(ctx context.Context, prompt string, config AgentConfig) (*RunHandle, error)

type AgentConfig struct {
    Model       llm.Model
    Permissions BashPermissions
    WorkDir     string
    Env         []string
    InputCh     <-chan string // Optional interactive input channel
    CacheRetention llm.CacheRetention // Prompt caching preference (default: unset/none)
}

type RunHandle struct {
    Events <-chan Event
}

func (h *RunHandle) Wait() (RunResult, error)
```

When `InputCh` is non-nil, the agent emits `WaitingForInputEvent` on natural
completion and waits for additional user input, ignoring whitespace-only lines.
See [agent.md](./agent.md) for event and result type definitions.
