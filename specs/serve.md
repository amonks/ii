# Serve

## Overview
The serve package runs pooled job workers alongside a merge loop to land completed
changes onto a target bookmark.

## API

```go
type RunLLMFunc func(job.AgentRunOptions) (job.AgentRunResult, error)

type TranscriptsFunc func(string, []job.AgentSession) ([]job.AgentTranscript, error)

type Options struct {
    Workers      int
    RepoPath     string
    Target       string
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
- `Workers` defaults to 4; `PollInterval` defaults to one second.
- `Target` defaults to `main`.
- Serve runs two loops concurrently:
  - A pool of job workers (`pool.Run`) for ready todos.
  - A merge loop that handles todos queued for merge.
- The merge loop:
  - Acquires a dedicated workspace from the workspace pool.
  - Polls for todos with status `queued_for_merge`.
  - Marks a todo as `merging` before attempting the merge.
  - Looks up the job by `JobID` to fetch the latest change ID.
  - Runs `merge.Merge` to rebase and advance the target bookmark.
  - Marks the todo as `done` on success.
  - Marks the todo as `merge_failed` on failure (canceled merges propagate the cancellation without updating status).
- The serve runner exits when the context is cancelled or one of the loops returns an error.
