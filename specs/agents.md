# Agents Package

## Overview

The `agents` package provides a thin abstraction over agent backends so job runs
can use either the built-in agent loop or shell out to external CLIs. It is
intentionally minimal and only supports the subset needed by `job`.

## Backends

- **internal**: wraps the `agent` package with persistence, event streaming, and
  configuration-aware model resolution.
- **claude**: shells out to `claude -p --dangerously-skip-permissions`,
  passing the prompt on stdin.
- **codex**: shells out to `codex exec --skip-git-repo-check`, passing the
  prompt on stdin.

Shell backends do not emit events or provide transcripts; they only return an
exit code and a synthetic session ID.

## Interfaces

```go
type RunOptions struct {
    RepoPath  string
    WorkDir   string
    Prompt    string
    Model     string
    StartedAt time.Time
    Version   string
    Env       []string
}

type RunResult struct {
    SessionID string
    ExitCode  int
    Error     string  // Error message when ExitCode is non-zero (optional; best-effort, depends on backend)
}

type RunHandle interface {
    Events() <-chan Event
    Wait() (RunResult, error)
}

type Runner interface {
    Run(context.Context, RunOptions) (RunHandle, error)
}
```

`ExitCode` is a result code indicating success (0) or failure (non-zero). For shell
backends (claude, codex), this is the actual process exit code. For the internal
backend, it is a synthetic code: 0 when the agent run completes without error, 1
when an error occurs. The term "exit code" is used for consistency across backends
even though the internal backend does not spawn a subprocess.

## Session IDs

External runners generate a session ID with the form
`external-<backend>-<hash>`, where the hash is a deterministic short ID derived
from the prompt and start timestamp.

## Transcripts

The internal transcript store remains provided by the `agent` package and is
exposed as `agents.OpenTranscriptStore()` for job runners.
