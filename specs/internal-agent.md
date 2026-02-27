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
type PromptContent struct {
    ProjectContext []string
    ContextFiles   []string
    TestCommands   []string
    PhaseContent   string
    UserContent    string
}

func Run(ctx context.Context, prompt PromptContent, config AgentConfig) (*RunHandle, error)

type AgentConfig struct {
    Model       llm.Model
    Permissions BashPermissions
    WorkDir     string
    Env         []string
    InputCh     <-chan string // Optional interactive input channel
    CacheRetention llm.CacheRetention // Prompt caching preference (default: short)
}

type RunHandle struct {
    Events <-chan Event
}

func (h *RunHandle) Wait() (RunResult, error)
```

When running, the agent loads project-level prompt context (workflow context, review
questions, default review instructions, context files, and test commands) from the repo and
renders the workflow/review templates before storing them in `PromptContent`.
`Run` only fills these fields when the corresponding `PromptContent` field is
empty, so callers can override or precompute prompt context without duplication.

Subagents load project-level prompt context (workflow context, review questions,
default review instructions, context files, test commands, and phase content) the same
way as parent agents. `runSubagent` uses the same repo-derived defaults when the
corresponding `PromptContent` fields are empty, so subagents inherit persistent
instructions unless the caller supplies explicit prompt context.

When `InputCh` is non-nil, the agent emits `WaitingForInputEvent` on natural
completion and waits for additional user input, ignoring whitespace-only lines.
See [agent.md](./agent.md) for event and result type definitions.
