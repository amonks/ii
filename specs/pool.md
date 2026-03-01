# Pool

## Overview
The pool package runs a configurable number of job workers that each acquire a
workspace from the workspace pool and process ready todos.

## API

```go
type RunLLMFunc func(job.AgentRunOptions) (job.AgentRunResult, error)

type TranscriptsFunc func(string, []job.AgentSession) ([]job.AgentTranscript, error)

type Options struct {
    Workers      int
    RepoPath     string
    RunLLM       RunLLMFunc
    Transcripts  TranscriptsFunc
    PollInterval time.Duration
    Now          func() time.Time
    LoadConfig   func(string) (*config.Config, error)
    RunTests     func(string, []string) ([]job.TestCommandResult, error)
    Model        string
    UpdateStale  func(string) error
    Snapshot     func(string) error
}

func Run(ctx context.Context, opts Options) error
```

## Behavior
- `Run` requires `RepoPath` and a configured `RunLLM`.
- `Workers` defaults to 4.
- `PollInterval` defaults to one second.
- Each worker:
  - Acquires a workspace for the repo.
  - Polls for ready todos (`todo.Ready(1)`); sleeps for `PollInterval` when none are available.
  - Marks the todo as `in_progress` before running a job.
  - Runs `job.Run` with `SkipFinalize` so the caller manages todo status.
  - On success, marks the todo as `queued_for_merge` and stores the job ID.
  - On failure, reopens the todo to `open`.
- The pool exits when the context is cancelled or a worker returns an error; when a
  worker returns a non-cancellation error, the pool cancels the context so other
  workers exit promptly.
- Workspaces are released when workers exit.
