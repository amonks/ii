# Resume Interrupted Jobs

## Goal

Add `ii job resume <job-id>` so that an interrupted job can be restarted from
the implementing stage, preserving all previously committed changes.

## Design decisions

- **Always resume at implementing.** Regardless of which stage was interrupted,
  the user abandons any in-progress working-copy changes before resuming, so
  the only valid entry point is implementing. The `reviewScope` and other
  in-memory state reconstruct naturally from the loop.
- **Workspace safety checks before resuming.** Resume fails if:
  1. `@` is not empty (`jj log -r @ -T empty` must be `true`).
  2. Prior committed changes are missing. Run one `jj log` with a revset like
     `ancestors(@) & (change1 | change2 | ...)` and verify all expected change
     IDs appear. Compare change IDs only, never commit IDs, so rebasing between
     failure and resume is fine.
- **Handle both clean and hard interrupts.** A SIGINT sets the job to `failed`;
  a SIGKILL or power loss leaves it `active` with a stale `UpdatedAt`. Resume
  accepts either state. In both cases the todo is already `in_progress`.
- **No prompt enrichment or conversation replay.** The agent gets the normal
  implementation prompt. The workspace and jj history reflect prior committed
  rounds, which is enough context.
- **Reuse the existing job record.** Resume flips status back to `active`, sets
  stage to `implementing`, and appends to the existing event log. No new job ID.
- **One resume at a time.** If the job is already `active` and its `UpdatedAt`
  is recent (within some threshold, e.g. 5 minutes), refuse to resume — it
  might still be running. This handles the SIGKILL case where status was never
  updated.

## Changes

### 1. Job runner: add `Resume` function

New public function in `job/runner.go`:

```go
func Resume(repoPath, jobID string, opts RunOptions) (*RunResult, error)
```

Steps:
- Load the job by ID via `Manager.Get`.
- Validate status is `failed` or `active`-but-stale.
- Run workspace safety checks (see below).
- Flip status to `active`, stage to `implementing`.
- Open existing event log in append mode.
- Call `runJobStages()` with the loaded job.
- On completion, finalize the todo as usual.

### 2. Job runner: fix `runJobStages` stage dispatch

Currently `runJobStages` asserts `current.Stage == StageImplementing` at the
top of the loop. Since resume always enters at implementing this isn't strictly
blocking, but the assertion should become a proper stage dispatch so the code
doesn't lie about what it supports. For now, resume sets stage to implementing
before calling `runJobStages`, so the existing assertion holds.

### 3. Workspace safety checks

New unexported function in `job/runner.go`:

```go
func checkWorkspaceForResume(workspacePath string, changes []JobChange) error
```

- Check `@` is empty: `jj log -r @ -T empty` returns `true`.
- Collect change IDs from all `IsComplete()` changes in `job.Changes`.
- If there are completed changes, run:
  `jj log --no-graph -r 'ancestors(@) & (id1 | id2 | ...)' -T 'change_id ++ "\n"'`
- Parse output, verify every expected change ID is present.
- Return a clear error listing any missing change IDs.

### 4. Event log: support append mode

`OpenEventLog` currently creates a new file. Add an option or a separate
`OpenEventLogAppend` that opens the existing file in append mode for resume.
The JSONL format is naturally appendable.

### 5. Manager: add `Get` method (if missing)

Verify the `Manager` can load a job by ID. If not, add a `Get(jobID string)
(Job, error)` method. Resume needs to load the full job record including
changes and agent sessions.

### 6. CLI: add `ii job resume` command

New file `cmd/ii/job_resume.go`:

- Takes one positional arg: job ID (prefix-matchable).
- Validates the job exists and is resumable.
- Streams output the same way `ii job do` does.
- Error messages guide the user: "working copy is not empty; run jj abandon
  first", "expected change xyz not found in history".

### 7. Staleness detection

For jobs left `active` by a hard kill, define a staleness threshold. If
`UpdatedAt` is older than the threshold, treat the job as resumable. If it's
recent, refuse with "job may still be running."

## Out of scope

- Conversation replay / reconstructing `[]llm.Message` from event logs.
- Prompt enrichment with transcripts of prior sessions.
- Resuming at stages other than implementing.
- Automatic cleanup of abandoned working-copy changes.
