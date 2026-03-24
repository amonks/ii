# Pool, Merge, and Serve

## Goal

Build three new modules that automate the workflow of running parallel job
workers in pooled workspaces and merging completed work onto a target bookmark.

## Modules

Three new public packages, each with a thin CLI wrapper, following the same
pattern as existing modules (job, workspace, todo, etc.):

1. **`merge/`** — Rebase a change onto a target bookmark, resolving conflicts
   with an agent.
2. **`pool/`** — Run N job workers in acquired workspaces, polling for ready
   todos.
3. **`serve/`** — Pool + a merge loop that lands completed work.

## Design decisions

### New todo statuses

Three new statuses in the todo lifecycle:

- **`queued_for_merge`** — Job completed successfully; changes exist in jj but
  haven't landed on the target bookmark yet.
- **`merging`** — The merge loop has picked this up and is actively rebasing /
  resolving conflicts.
- **`merge_failed`** — Merge was attempted but failed (even after agent conflict
  resolution). Requires human intervention.

Status flow:

```
open → in_progress → queued_for_merge → merging → done
                                                 ↘ merge_failed
```

Recovery from `merge_failed`: human investigates, then either manually resolves
and marks `done`, transitions back to `queued_for_merge` to retry, or reopens to
`open` for reimplementation.

**Dependency resolution**: None of the three new statuses are "resolved" for
dependency purposes. A dependent todo only becomes ready when its dependency
reaches `done` (i.e. changes have landed on the target bookmark). This is
correct because workers start jobs with `jj new main`, so they need dependencies'
changes to be on main.

**Timestamps**: `startedAt` and `completedAt` validation must be extended to
allow these statuses to preserve timing through the merge pipeline. Set
`completedAt` when entering `queued_for_merge` from `in_progress` (this is when
implementation work finished). Preserve it through `merging` → `done`.

### Job ID on todo

Add a `JobID string` field to the todo model. Set when a job completes and the
todo transitions to `queued_for_merge`. The merge loop uses this to look up the
job's changes (via the job-changes system) and find the tip change ID for
rebasing.

### Job finalization

Currently `job.Run` hardcodes todo finalization: `done` on success, `open` on
failure. Pool/serve need different post-job behavior (`queued_for_merge` on
success).

Add `SkipFinalize bool` to `job.RunOptions`. When set, `job.Run` skips the
`finalizeTodo` call, letting the caller manage the todo lifecycle. The existing
`job.Run` behavior (without the flag) is unchanged.

### Merge mechanics (jj operations)

The merge operation for a change ID:

1. Rebase the change's ancestry onto the target bookmark:
   `jj rebase -b <change_id> -d <target>` (rebases the branch including the
   change, preserving structure).
2. Detect conflicts: `jj log --no-graph -r <change_id>::<target_tip> -T conflict`
   to find conflicted revisions in the rebased chain.
3. For each conflicted revision (in topological order):
   - `jj new <conflicted_rev>` to create a resolution change.
   - Run an agent with instructions to resolve conflict markers.
   - Squash the resolution into the conflicted change (`jj squash`).
   - Re-check for conflicts.
   - If still conflicted after resolution attempt, fail.
4. On success: `jj bookmark set <target> -r <tip>` to advance the bookmark.

### jj wrapper additions

New methods needed on `jj.Client`:

| Method | Command | Purpose |
|--------|---------|---------|
| `Rebase(wsPath, rev, dest)` | `jj rebase -b <rev> -d <dest>` | Rebase branch onto destination |
| `HasConflicts(wsPath, rev)` | `jj log --no-graph -r <rev> -T conflict` | Check if a revision has conflicts |
| `ConflictedInRange(wsPath, revset)` | `jj log --no-graph -r <revset> -T 'if(conflict, change_id)'` | Find conflicted changes in a range |
| `BookmarkSet(wsPath, name, rev)` | `jj bookmark set <name> -r <rev>` | Move/create bookmark at revision |
| `Squash(wsPath)` | `jj squash` | Squash current change into parent |

### Pool behavior

Pool is long-running (polls for work, blocks until context cancellation /
signal). Each worker:

1. Acquires a workspace from the pool at startup.
2. Loop: query `todo.Ready(1)`, if empty sleep briefly and retry.
3. `jj new main` in the workspace to start from current main.
4. Call `job.Run` with `SkipFinalize: true`.
5. On success: set todo status to `queued_for_merge`, store job ID on todo.
6. On failure: reopen todo to `open`.
7. Repeat from step 2.
8. On shutdown: release all workspaces.

The pool package API:

```go
type Options struct {
    Workers    int
    RepoPath   string
    RunLLM     job.RunLLMFunc    // LLM runner (from CLI layer)
    Transcripts job.TranscriptsFunc
    // ... other job.RunOptions fields passed through
}

func Run(ctx context.Context, opts Options) error
```

### Serve behavior

Serve is long-running. Starts a pool and a merge loop concurrently
(`errgroup`).

The merge loop:

1. Poll for todos with status `queued_for_merge`.
2. Pick the first one, transition to `merging`.
3. Look up the job ID → job → tip change ID.
4. Acquire a workspace for merging (one dedicated merge workspace, reused).
5. Call `merge.Merge(ctx, opts)`.
6. On success: transition todo to `done`.
7. On failure: transition todo to `merge_failed`.
8. Repeat.

Serve package API:

```go
type Options struct {
    Workers    int
    RepoPath   string
    Target     string  // bookmark to merge onto; default "main"
    // ... LLM / job options passed through to pool
}

func Run(ctx context.Context, opts Options) error
```

### Merge package API

```go
type Options struct {
    RepoPath      string
    WorkspacePath string
    ChangeID      string
    Target        string   // bookmark name, e.g. "main"
    RunLLM        func     // for conflict resolution agent
}

func Merge(ctx context.Context, opts Options) error
```

The merge package is stateless — it does one merge and returns. All persistence
(todo status, job tracking) is handled by the caller (serve's merge loop).

### CLI commands

```
ii merge <change-id> [--onto main]      # standalone merge operation
ii pool [--workers=N]                   # run job workers
ii serve [--workers=N] [--onto main]    # pool + merge loop
```

`ii merge` operates in the current jj workspace (like `ii job do`). Useful for
manually merging a change. Does not interact with the todo system.

`ii pool` and `ii serve` manage their own workspaces via the workspace pool.

### Config additions

```toml
[pool]
workers = 4        # default number of workers

[merge]
target = "main"    # default target bookmark
```

CLI flags override config values.

## Integration points

### Todo package (`todo/`)

1. **`types.go`**: Add `StatusQueuedForMerge`, `StatusMerging`, `StatusMergeFailed`
   constants. Add to `ValidStatuses()`. Do NOT add to `IsResolved()`.
2. **`todo.go`**: Add `JobID string` field.
3. **`operations.go`**: Add new statuses to the active-group `case` in
   `applyStatusChange` (clear `ClosedAt`). Set `CompletedAt` when entering
   `queued_for_merge` from `in_progress`. Preserve `StartedAt` through merge
   statuses.
4. **`validation.go`**: Extend `validateStartedAt` to allow `startedAt` for the
   three new statuses. Extend `validateCompletedAt` similarly. Add `JobID`
   validation (only valid for merge-related statuses).
5. **`store.go`**: Add `QueueForMerge(ids []string, jobID string)` convenience
   method. Add `Merge(ids []string)` and `MergeFailed(ids []string)` convenience
   methods.
6. **Spec**: Update `specs/todo.md` with new statuses, fields, transitions.

### Job package (`job/`)

1. **`runner.go`**: Add `SkipFinalize bool` to `RunOptions`. When set, skip
   `startTodo` and `finalizeTodo` calls — the caller manages todo lifecycle.
2. **`types.go`**: Ensure `Job.Changes` and the tip change ID are accessible via
   public API (they already are via `Job.Changes` slice).
3. **Spec**: Update `specs/job.md` with `SkipFinalize` option.

### jj wrapper (`internal/jj/`)

1. **`jj.go`**: Add `Rebase`, `HasConflicts`, `ConflictedInRange`,
   `BookmarkSet`, `Squash` methods.
2. **Spec**: Update `specs/internal-jj.md`.

### Config (`internal/config/`)

1. **`config.go`**: Add `Pool` and `Merge` config sections.
2. **Spec**: Update `specs/internal-config.md`.

### CLI (`cmd/ii/`)

1. **`merge.go`**: New file. `ii merge` command, thin wrapper around
   `merge.Merge`.
2. **`pool.go`**: New file. `ii pool` command, thin wrapper around `pool.Run`.
3. **`serve.go`**: New file. `ii serve` command, thin wrapper around
   `serve.Run`.
4. **`main.go`**: Register new top-level commands.
5. **Spec**: Update `specs/cli.md`.

### New specs

1. **`specs/merge.md`**: Merge package specification.
2. **`specs/pool.md`**: Pool package specification.
3. **`specs/serve.md`**: Serve package specification.

## Implementation order

Each step is a complete vertical slice: package + CLI + spec + tests. CI passes
after each step.

1. **jj wrapper additions** — Add `Rebase`, `HasConflicts`, `ConflictedInRange`,
   `BookmarkSet`, `Squash` methods with unit tests.
   → update `specs/internal-jj.md`

2. **Todo status & field additions** — Add `queued_for_merge`, `merging`,
   `merge_failed` statuses and `JobID` field. Add convenience methods
   (`QueueForMerge`, `MergeFailed`). Update validation, `applyStatusChange`.
   Unit tests for all new transitions and edge cases.
   → update `specs/todo.md`

3. **Job `SkipFinalize`** — Add the option, test that it skips todo lifecycle
   management.
   → update `specs/job.md`

4. **`merge` module** — `merge/` package, `ii merge` CLI command, config
   `[merge] target`, e2e testscript under `cmd/ii/testdata`.
   → create `specs/merge.md`, update `specs/cli.md`, `specs/internal-config.md`,
     add to `specs/README.md` table

5. **`pool` module** — `pool/` package, `ii pool` CLI command, config
   `[pool] workers`, e2e testscript under `cmd/ii/testdata`.
   → create `specs/pool.md`, update `specs/cli.md`, `specs/internal-config.md`,
     add to `specs/README.md` table

6. **`serve` module** — `serve/` package, `ii serve` CLI command, e2e testscript
   under `cmd/ii/testdata`.
   → create `specs/serve.md`, update `specs/cli.md`, add to `specs/README.md`
     table
