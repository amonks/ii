# Internal State

## Overview
The state package manages the legacy JSON state file (`~/.local/state/incrementum/state.json`) that persists repo metadata and job history. Agent sessions are now stored in SQLite via `internal/db` and are referenced from jobs by ID.

## State File Structure
The state file contains:
- `repos`: maps repo names to source paths (used by job tracking)
- `jobs`: maps job ids to job records

## Types

### Job
- `id`, `repo`, `todo_id`, `stage`, `feedback`, `agent`, `agent_sessions`, `status`, `created_at`, `started_at`, `updated_at`, `completed_at`
- `agent_sessions`: list of session references (`JobAgentSession`) with purpose and session ID
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
- `GetOrCreateRepoName(path)`: get or create repo name for path; normalizes paths (resolves symlinks) for consistent matching across platforms
- `SanitizeRepoName(path)`: convert path to safe repo name
