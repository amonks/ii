# Merge

## Overview
The merge package rebases a change onto a target bookmark and resolves conflicts
with a conflict-resolution agent.

## API

```go
type RunLLMFunc func(job.AgentRunOptions) (job.AgentRunResult, error)

type Options struct {
    RepoPath      string
    WorkspacePath string
    ChangeID      string
    Target        string
    RunLLM        RunLLMFunc
    Now           func() time.Time
}

func Merge(ctx context.Context, opts Options) error
```

## Behavior
- `Merge` requires `RepoPath`, `WorkspacePath`, and `ChangeID`.
- `Target` defaults to `main` when unset.
- If `WorkspacePath` is empty, it defaults to `RepoPath`.
- `Merge` rebases the change onto the target bookmark using `jj rebase -b <change> -d <target>`.
- Conflicts are detected using `jj log --no-graph -r <target>::<change> -T 'if(conflict, change_id)'`.
- When conflicts are detected, `RunLLM` must be provided to resolve them.
- Conflict resolution loop:
  - For each conflicted change, create a resolution change (`jj new <change>`).
  - Run the conflict-resolution agent.
  - Snapshot workspace changes and squash (`jj squash`) into the conflicted change.
  - Re-check for conflicts; if conflicts remain after all attempts, return an error.
- On success, advance the target bookmark with `jj bookmark set <target> -r <change>`.
- After advancing the bookmark, the merge verifies no conflicts remain before returning.

## Conflict Resolution Prompt
- The prompt instructs the agent to resolve conflict markers and avoid unrelated changes.
- Context files (`AGENTS.md` or `CLAUDE.md`) from the repository and global config are included.
