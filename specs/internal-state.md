# Internal State

## Overview
The state package manages the legacy JSON state file (`~/.local/state/incrementum/state.json`) for agent sessions and jobs. Workspace data now lives in SQLite via `internal/db`.

## State File Structure
The state file contains:
- `repos`: maps repo names to source paths (used by agent/job)
- `agent_sessions`: maps session keys to agent session records
- `jobs`: maps job ids to job records

## Types

### AgentSession
- `id`, `repo`, `status`, `model`, `created_at`, `started_at`, `updated_at`, `completed_at`, `exit_code`, `duration_seconds`, `tokens_used`, `cost`
- Status: `active`, `completed`, or `failed`
- Note: Prompts are not stored to keep the state file small; they can be reconstructed from job/todo context

### Job
- `id`, `repo`, `todo_id`, `stage`, `feedback`, `agent`, `agent_sessions`, `status`, `created_at`, `started_at`, `updated_at`, `completed_at`
- `changes`: list of `JobChange` tracking changes created during the job
- `project_review`: final project review outcome (`JobReview`)
- Stage: `implementing`, `testing`, `reviewing`, or `committing`
- Status: `active`, `completed`, `failed`, or `abandoned`

See [job-changes.md](./job-changes.md) for details on `JobChange`, `JobCommit`, and `JobReview` types.

## Locking
All state updates use advisory file locking via `state.lock` to serialize concurrent access from multiple processes.

## API
- `NewStore(dir)`: create a store for the given directory
- `Load()`: read current state
- `Save(state)`: write state atomically, skipping disk writes when no changes
- `Update(fn)`: read-modify-write with locking
- `GetOrCreateRepoName(path)`: get or create repo name for path; normalizes paths (resolves symlinks) for consistent matching across platforms. Workspace resolution now happens in SQLite.
- `SanitizeRepoName(path)`: convert path to safe repo name
