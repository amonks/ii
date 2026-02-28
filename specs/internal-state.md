# Internal State

## Overview
The `internal/state` package manages the legacy JSON state file
(`~/.local/state/incrementum/state.json`). This file is no longer the active
source of truth; it exists only for migration into the SQLite database via
`internal/db`. New job and repo metadata are stored in SQLite, and the legacy
JSON file is renamed to `state.json.bak` once migrated.

## State File Structure
The legacy state file contains:
- `repos`: maps repo names to source paths
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

## Legacy Usage
- The JSON store is read only during migration to SQLite (`internal/db`).
- Production code should read/write job and agent session state via SQLite tables.

## API
- `NewStore(dir)`: create a legacy JSON store for the given directory
- `Load()`: read current legacy state
- `Save(state)`: write legacy state atomically, skipping disk writes when no changes
- `Update(fn)`: read-modify-write with advisory file locking (legacy use only)
- `GetOrCreateRepoName(path)`: get or create repo name for path; normalizes paths (resolves symlinks) for consistent matching across platforms
- `SanitizeRepoName(path)`: convert path to safe repo name
