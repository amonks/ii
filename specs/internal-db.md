# Internal DB

## Overview
The `internal/db` package owns the SQLite connection for incrementum state. It opens the single state database, applies migrations, and configures SQLite pragmas. On first run, it can migrate legacy JSON state into SQLite.

## Responsibilities
- Open the SQLite database at the configured path.
- Apply embedded SQL migrations in version order.
- Maintain the `schema_version` table.
- Configure recommended pragmas (WAL, busy timeout, foreign keys, etc.).
- Migrate legacy `state.json` data into SQLite when requested.

## API
- `Open(path, opts)` returns a `*DB` wrapper with the configured connection.
- `DB.Close()` closes the connection.
- `DB.Tx(fn)` runs a callback in a transaction.
- `DB.SqlDB()` exposes the underlying `*sql.DB` for domain stores.
- `GetOrCreateRepoName(db, path)` normalizes and stores repo slugs in the `repos` table, handling collisions.
- `RepoNameForPath(db, path)` resolves the repo slug for a known source path, returning empty string if none exists.
- `RepoPathForWorkspace(db, wsPath)` resolves a workspace path back to its source repo using the `workspaces` table.
- `SanitizeRepoName(path)` converts paths into slug-safe repo names.

## Migrations
Migration files live in `internal/db/migrations` and are embedded. Files are named with a numeric prefix (`001_*.sql`, `002_*.sql`, ...). The `schema_version` table tracks the current version.

## Legacy JSON schema
The legacy `state.json` file stores repository and job metadata to support migration into SQLite. It includes:

- `repos`: map of repo names to `{source_path}`.
- `workspaces`: map of workspace keys to workspace metadata.
- `agent_sessions`: map of session keys to agent session metadata.
- `jobs`: map of job IDs to job records, including changes, commits, and reviews.

This schema is only used by the JSON migration routine; runtime access is exclusively through SQLite.

If `OpenOptions.LegacyJSONPath` is set and the database file does not exist yet, `Open` prompts to migrate data from `state.json` unless `SkipConfirm` is true. When confirmed, it inserts data into the new schema and renames `state.json` to `state.json.bak`.

## Pragmas
On open, the database is configured with:

```sql
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -2000;
```
