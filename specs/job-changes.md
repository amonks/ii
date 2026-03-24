# Job Change Tracking

## Overview

This spec describes enhanced persistence for job execution state, reifying
ephemeral stack-based state into the job record. The goal is to enable richer
observability (seeing the iterations that led to each change, debugging
abandonment cases) and lay groundwork for future features like resumption.

This is orthogonal to the event log: the event log is for debugging and replay,
while this structured state is for application logic and UI. The event log
format may change independently.

## Motivation

Currently, job execution state lives primarily in the call stack:

- Which changes have been committed
- The current work-in-progress (draft message, test results)
- Review history for each change (rejections, iterations)

This state is lost on crash and unavailable to other processes. We want to:

1. Show rich job status in `ii job list` and other UIs (kanban-style boards)
2. Enable drilling into a job to see each change, its commits, and review history
3. Debug abandonment cases by seeing what iterations were attempted

## Data Model

### Job (extended)

Add to the existing `Job` struct in `job/types.go`:

```go
type Job struct {
    // ... existing fields ...

    // Changes created by this job, in order of creation.
    // Each change may have multiple commits (from review/implement iterations).
    Changes []JobChange `json:"changes,omitempty"`

    // ProjectReview captures the final project review (after all changes complete).
    // Nil until project review runs.
    ProjectReview *JobReview `json:"project_review,omitempty"`
}
```

### JobChange

A change corresponds to a jj change (with a stable change ID). As the job
iterates through implement/test/review cycles, each iteration produces a new
commit within the same change. Eventually one commit is accepted and the change
is complete.

```go
// JobChange represents a change being built up during a job.
// Maps to a jj change (stable change ID across rebases).
type JobChange struct {
    // ChangeID is the jj change ID (stable across rebases).
    ChangeID string `json:"change_id"`

    // Commits is the sequence of commits for this change.
    // Each implement/review iteration appends a new commit.
    // The last commit with an accepted review is the final commit.
    Commits []JobCommit `json:"commits"`

    // CreatedAt is when this change was first created.
    CreatedAt time.Time `json:"created_at"`
}
```

The iteration count for a change is `len(change.Commits)`.

A change is complete when its last commit has a review with outcome `ACCEPT`.

For derived-state purposes, a change is considered "current" when it is not
complete (i.e. its last commit is not accepted). This includes:

- no commits yet
- last commit has no review
- last review outcome is not `ACCEPT` (e.g. `REQUEST_CHANGES` or `ABANDON`)

Whether further work is allowed is gated by job status/stage: `REQUEST_CHANGES`
loops back to implementation, while `ABANDON` ends the job even though the last
change remains not complete in history.

### JobCommit

Each implement/test/review pass produces a commit. The commit may be reviewed
and either accepted (completing the change) or rejected (triggering another
iteration).

```go
// JobCommit represents one commit within a change.
// Each implement/review iteration produces a new commit.
type JobCommit struct {
    // CommitID is the jj commit ID for this iteration.
    CommitID string `json:"commit_id"`

    // DraftMessage is the commit message from the implementing stage.
    DraftMessage string `json:"draft_message"`

    // TestsPassed indicates whether tests passed for this commit.
    // Only meaningful after the testing stage completes.
    // This is intentionally minimal (pass/fail only); logs stay in the event log.
    TestsPassed *bool `json:"tests_passed,omitempty"`

    // Review is the review decision for this commit.
    // Nil until the reviewing stage completes.
    Review *JobReview `json:"review,omitempty"`

    // AgentSessionID references the agent session that produced this commit.
    AgentSessionID string `json:"agent_session_id"`

    // CreatedAt is when this commit was created.
    CreatedAt time.Time `json:"created_at"`
}
```

### JobReview

Captures a review decision, whether for a commit or for the final project
review.

```go
// JobReview captures a review decision.
type JobReview struct {
    // Outcome is the review verdict: ACCEPT, REQUEST_CHANGES, or ABANDON.
    Outcome ReviewOutcome `json:"outcome"`

    // Comments is the reviewer's feedback text.
    // Present for all outcomes; may be empty for accept.
    Comments string `json:"comments,omitempty"`

    // AgentSessionID references the agent session that produced this review.
    AgentSessionID string `json:"agent_session_id"`

    // ReviewedAt is when the review was recorded.
    ReviewedAt time.Time `json:"reviewed_at"`
}
```

## State Transitions

### Implementing Stage

On entering the implementing stage:

1. Check if there's an in-progress change (last change where last commit has no
   accepted review).
2. If no in-progress change exists, create a new `JobChange` with a generated
   `ChangeID` and empty `Commits` slice.
3. Record the `BeforeCommitID` for change detection.

On exiting the implementing stage (with changes detected):

1. Create a new `JobCommit` with:
   - `CommitID`: current working copy commit ID
   - `DraftMessage`: from `.incrementum-commit-message`
   - `AgentSessionID`: the agent session that just ran
   - `CreatedAt`: now
2. Append the commit to the current change's `Commits` slice.

On exiting the implementing stage (no changes detected):

1. Do not modify `Changes`.
2. Proceed to project review.

### Testing Stage

On test completion:

1. Find the current commit (last commit of last change).
2. Set `TestsPassed` to `true` if all tests passed, `false` otherwise.

### Reviewing Stage (Step Review)

On review completion:

1. Find the current commit (last commit of last change).
2. Create a `JobReview` with the outcome, comments, and session ID.
3. Set `commit.Review` to the new review.

If outcome is `ACCEPT`:
- The change is now complete.
- Proceed to committing stage.

If outcome is `REQUEST_CHANGES`:
- The change remains in-progress.
- Return to implementing stage (next iteration will append a new commit).

If outcome is `ABANDON`:
- Job ends with `abandoned` status.

### Reviewing Stage (Project Review)

On project review completion:

1. Create a `JobReview` with the outcome, comments, and session ID.
2. Set `Job.ProjectReview` to the new review.

If outcome is `ACCEPT`:
- Job ends with `completed` status.

If outcome is `REQUEST_CHANGES`:
- Return to implementing stage (will start a new change).

If outcome is `ABANDON`:
- Job ends with `abandoned` status.

### Committing Stage

On successful commit:

1. The current change is already marked complete (review accepted).
2. The `CommitID` on the last commit is the final commit ID.
3. No additional state updates needed.

Return to implementing stage to potentially start a new change.

## Deriving State

### Current Change

The current in-progress change is:

```go
func (j *Job) CurrentChange() *JobChange {
    if j == nil || len(j.Changes) == 0 {
        return nil
    }
    last := &j.Changes[len(j.Changes)-1]
    if last.IsComplete() {
        return nil
    }
    return last
}

func (c JobChange) IsComplete() bool {
    if len(c.Commits) == 0 {
        return false
    }
    last := c.Commits[len(c.Commits)-1]
    return last.Review != nil && last.Review.Outcome == ReviewOutcomeAccept
}
```

### Current Commit

The current in-progress commit is the last commit of the current change:

```go
func (j *Job) CurrentCommit() *JobCommit {
    change := j.CurrentChange()
    if change == nil || len(change.Commits) == 0 {
        return nil
    }
    return &change.Commits[len(change.Commits)-1]
}
```

### Iteration Count

The iteration count for the current change is `len(change.Commits)`.

## UI Implications

### `ii job list`

Could show change count and current iteration:

```
JOB      TODO     STAGE         STATUS  CHANGES  ITERATION  AGE
abc123   xy34     reviewing     active  2        3          5m
def456   mn78     implementing  active  0        1          2m
```

### `ii job show <id>`

Could show change history:

```
Job abc123 - "Add dark mode toggle"
Status: active (reviewing)

Changes:
  [1] kpqvwx (3 iterations)
      Commit a1b2c3: tests passed, review: REJECTED
        "The constants should be in a separate file"
      Commit d4e5f6: tests passed, review: REJECTED
        "Still mixing concerns, extract to theme.go"
      Commit g7h8i9: tests passed, review: ACCEPTED
        "Clean separation, good naming"

  [2] lmnoyz (1 iteration, in progress)
      Commit j0k1l2: tests passed, reviewing...
```

### Kanban Board (Future UI)

Changes can be visualized as cards moving through columns:

- **Implementing**: changes where last commit has no review
- **Testing**: changes where last commit has `TestsPassed == nil`
- **Reviewing**: changes where last commit has `TestsPassed` but no review
- **Committed**: changes where last commit has accepted review

Drilling into a change shows the commit history as its own kanban:

- **Draft**: commits with no test result
- **Testing**: commits with `TestsPassed == nil`
- **Reviewing**: commits with test result but no review
- **Rejected**: commits with rejected review
- **Accepted**: the final commit (if any)

## Migration

No migration needed. Existing jobs will have empty `Changes` and `nil`
`ProjectReview`. The runner will populate these fields for new job runs.

Old jobs without change tracking remain valid; the UI should handle missing
data gracefully (e.g., show "N/A" for change count).

## Multiple Jobs Per Todo

Multiple jobs may target the same todo (e.g., after a failure and retry). Each
job maintains its own `Changes` list. This is intentional: the failed job's
change history is preserved for debugging, and the new job starts fresh.

## Out of Scope

- **SQLite writes**: Job changes are persisted via the SQLite store; write atomicity is handled by SQLite transactions.

- **Test output storage**: Test logs are intentionally excluded from the job
  record to avoid unbounded growth. They remain in the event log.

- **Change ID generation**: The spec assumes jj provides the change ID. The
  implementation will need to capture this from jj after the first commit.

## Implementation Status

This spec is fully implemented. The following components are in place:

- **Data model**: `JobChange`, `JobCommit`, `JobReview` in `job/types.go`
- **Derived state**: `CurrentChange()`, `CurrentCommit()`, `IsComplete()` methods on Job
- **Manager API**: `AppendChange`, `AppendCommitToCurrentChange`, `UpdateCurrentCommit`,
  `SetProjectReview` methods in `job/manager.go`
- **Runner integration**: State transitions wired into `runImplementingStage`,
  `runTestingStage`, and `runReviewingStage` in `job/runner.go`
- **Test coverage**: Integration tests in `job/runner_test.go` and unit tests in
  `job/manager_test.go`

## Implementation Notes

### Capturing Change ID

The change ID is captured via `opts.CurrentChangeID(workspacePath)` in the runner,
which calls `jj log -r @ -T 'change_id' --no-graph` via the jj client.

### Backward Compatibility

The runner handles jobs that were created before this feature:

- If `Changes` is nil/empty but the job has sessions, it's a legacy job.
- Legacy jobs can still complete; they just won't have change tracking.

### Manager API

The `job.Manager` provides dedicated methods (chosen over UpdateOptions fields
for clarity):

```go
type UpdateOptions struct {
    // ... existing fields ...

    // AppendChange adds a new change to the job.
    AppendChange *JobChange

    // AppendCommitToCurrentChange adds a commit to the last change.
    AppendCommitToCurrentChange *JobCommit

    // UpdateCurrentCommit updates the last commit of the last change.
    UpdateCurrentCommit *JobCommitUpdate

    // SetProjectReview sets the project review.
    SetProjectReview *JobReview
}

type JobCommitUpdate struct {
    TestsPassed *bool
    Review      *JobReview
}
```

Alternatively, provide dedicated methods:

```go
func (m *Manager) AppendChange(jobID string, change JobChange, now time.Time) (Job, error)
func (m *Manager) AppendCommit(jobID string, commit JobCommit, now time.Time) (Job, error)
func (m *Manager) UpdateCommit(jobID string, update JobCommitUpdate, now time.Time) (Job, error)
func (m *Manager) SetProjectReview(jobID string, review JobReview, now time.Time) (Job, error)
```
