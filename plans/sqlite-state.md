# SQLite State Store

## Goal

Replace the JSON state file (`state.json`) with a SQLite database. Each core
package owns its domain types and queries; a central `internal/db` package
manages the connection, migrations, and SQLite best-practices configuration.

## Design decisions

- **Single db file.** All repos share one database at
  `~/.local/state/incrementum/state.db`. Every top-level table includes
  `repo` as the first component of its primary key, matching the current
  JSON key structure. This avoids the chicken-and-egg problem of needing to
  resolve repo names before opening a db.
- **`internal/db` is the central coordinator.** It opens the database with WAL
  mode, busy timeout, foreign keys, and other production pragmas. It embeds
  migration SQL files and runs them on `Open()`. It exposes a `*DB` wrapper
  (or raw `*sql.DB`) to consumers.
- **Each core package owns its schema and types.** `workspace`, `agent`, and
  `job` each define their own domain types and have a store/repo type with
  query methods that accept the db handle. Migration SQL files live centrally
  in `internal/db/migrations/` with global sequential numbering, but each
  file's DDL corresponds to a single domain.
- **`agent` owns agent sessions; `job` references them by ID.** The `job`
  package imports `agent` (this import direction already exists). The
  `agent_sessions` table has a primary key that `job`-owned tables can
  foreign-key into.
- **Todos stay in jj.** The todo package's jj-workspace-based storage is
  unchanged.
- **Event/session logs stay as append-only JSONL files on disk.** No change to
  `~/.local/share/incrementum/agent/events/` or
  `~/.local/share/incrementum/jobs/events/`.
- **The lock file is removed.** SQLite's own locking (WAL mode) replaces
  `state.lock` and `syscall.Flock`.
- **JSON-to-SQLite migration on first run.** On `Open()`, if the db doesn't
  exist but `state.json` does, prompt the user to confirm, then import data
  into the new schema and rename `state.json` to `state.json.bak`.
- **Pure Go SQLite.** Use `modernc.org/sqlite` — no cgo dependency.
- **Migration versioning.** A `schema_version` table tracks the current
  version. Migrations are embedded SQL files named `001_*.sql`, `002_*.sql`,
  etc., applied in order on `Open()`.

## SQLite pragmas

Applied on every `Open()`:

```sql
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;
PRAGMA synchronous = NORMAL;   -- safe with WAL
PRAGMA cache_size = -2000;     -- 2MB
```

## Schema (initial migration `001_initial.sql`)

```sql
-- Repos (replaces the Repos map in state.json)
CREATE TABLE repos (
    name        TEXT PRIMARY KEY,
    source_path TEXT NOT NULL UNIQUE
);

-- Workspaces
CREATE TABLE workspaces (
    repo            TEXT NOT NULL REFERENCES repos(name),
    name            TEXT NOT NULL,
    path            TEXT NOT NULL,
    purpose         TEXT NOT NULL DEFAULT '',
    rev             TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'available'
        CHECK (status IN ('available', 'acquired')),
    acquired_by_pid INTEGER,
    provisioned     INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    acquired_at     TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (repo, name)
);

-- Agent sessions
CREATE TABLE agent_sessions (
    repo             TEXT NOT NULL REFERENCES repos(name),
    id               TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed', 'failed')),
    model            TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    started_at       TEXT NOT NULL DEFAULT '',
    updated_at       TEXT NOT NULL,
    completed_at     TEXT NOT NULL DEFAULT '',
    exit_code        INTEGER,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    tokens_used      INTEGER NOT NULL DEFAULT 0,
    cost             REAL NOT NULL DEFAULT 0,
    PRIMARY KEY (repo, id)
);

-- Jobs
CREATE TABLE jobs (
    repo                 TEXT NOT NULL REFERENCES repos(name),
    id                   TEXT NOT NULL,
    todo_id              TEXT NOT NULL,
    agent                TEXT NOT NULL DEFAULT '',
    implementation_model TEXT NOT NULL DEFAULT '',
    code_review_model    TEXT NOT NULL DEFAULT '',
    project_review_model TEXT NOT NULL DEFAULT '',
    stage                TEXT NOT NULL DEFAULT 'implementing'
        CHECK (stage IN ('implementing', 'testing', 'reviewing', 'committing')),
    status               TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed', 'failed', 'abandoned')),
    feedback             TEXT NOT NULL DEFAULT '',
    project_review_outcome          TEXT CHECK (project_review_outcome IN ('ACCEPT', 'REQUEST_CHANGES', 'ABANDON')),
    project_review_comments         TEXT NOT NULL DEFAULT '',
    project_review_agent_session_id TEXT NOT NULL DEFAULT '',
    project_review_reviewed_at      TEXT NOT NULL DEFAULT '',
    created_at           TEXT NOT NULL,
    started_at           TEXT NOT NULL DEFAULT '',
    updated_at           TEXT NOT NULL,
    completed_at         TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (repo, id)
);

-- Job-to-agent-session link (ordered list)
CREATE TABLE job_agent_sessions (
    repo       TEXT NOT NULL,
    job_id     TEXT NOT NULL,
    session_id TEXT NOT NULL,
    purpose    TEXT NOT NULL DEFAULT '',
    position   INTEGER NOT NULL,
    PRIMARY KEY (repo, job_id, session_id),
    FOREIGN KEY (repo, job_id) REFERENCES jobs(repo, id),
    FOREIGN KEY (repo, session_id) REFERENCES agent_sessions(repo, id)
);

-- Job changes (one per jj change)
CREATE TABLE job_changes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    repo       TEXT NOT NULL,
    job_id     TEXT NOT NULL,
    change_id  TEXT NOT NULL,
    created_at TEXT NOT NULL,
    position   INTEGER NOT NULL,
    FOREIGN KEY (repo, job_id) REFERENCES jobs(repo, id)
);
CREATE INDEX idx_job_changes_job ON job_changes(repo, job_id);

-- Job commits (one per iteration within a change)
CREATE TABLE job_commits (
    id                        INTEGER PRIMARY KEY AUTOINCREMENT,
    job_change_id             INTEGER NOT NULL REFERENCES job_changes(id),
    commit_id                 TEXT NOT NULL,
    draft_message             TEXT NOT NULL DEFAULT '',
    tests_passed              INTEGER,  -- NULL = unknown, 0 = failed, 1 = passed
    agent_session_id          TEXT NOT NULL DEFAULT '',
    review_outcome            TEXT CHECK (review_outcome IN ('ACCEPT', 'REQUEST_CHANGES', 'ABANDON')),
    review_comments           TEXT NOT NULL DEFAULT '',
    review_agent_session_id   TEXT NOT NULL DEFAULT '',
    review_reviewed_at        TEXT NOT NULL DEFAULT '',
    created_at                TEXT NOT NULL,
    position                  INTEGER NOT NULL
);
CREATE INDEX idx_job_commits_change ON job_commits(job_change_id);
```

Notes:
- Timestamps stored as RFC 3339 text (SQLite has no native datetime; text
  sorts correctly and is human-readable).
- Booleans stored as INTEGER (0/1) per SQLite convention.
- Every top-level entity has `repo` as the first component of its primary
  key. Child tables (job_changes, job_commits) inherit repo transitively
  through their parent foreign keys.
- `position` columns preserve ordering for lists (agent sessions, changes,
  commits).
- Reviews are inlined as columns rather than in a separate table. Both
  commit reviews and project reviews are 1:1 relationships, so a join table
  would add complexity without benefit. Commit reviews are nullable columns
  on `job_commits`; the project review is nullable columns on `jobs`.
- On the Go side, a shared `Review` struct in the `job` package (with
  `Outcome`, `Comments`, `AgentSessionID`, `ReviewedAt`) is composed into
  both `Job` (as `*Review` for project review) and `JobCommit` (as
  `*Review`). Scan/insert helpers map it to/from the `review_*` or
  `project_review_*` column prefixes as appropriate.

## `internal/db` package API

```go
package db

import "embed"

//go:embed migrations/*.sql
var migrations embed.FS

type DB struct {
    sql *sql.DB
}

// Open opens (or creates) the database at path
// (~/.local/state/incrementum/state.db by default), applies pending
// migrations, and configures pragmas. If LegacyJSONPath is set and the db
// doesn't exist yet, prompts the user and imports from the legacy JSON file.
func Open(path string, opts OpenOptions) (*DB, error)

// OpenOptions configures Open behavior.
type OpenOptions struct {
    // If non-empty, path to legacy state.json for one-time migration.
    LegacyJSONPath string
    // If true, skip the interactive confirmation prompt (for tests).
    SkipConfirm bool
}

// Close closes the database connection.
func (db *DB) Close() error

// Tx runs fn within a transaction.
func (db *DB) Tx(fn func(tx *sql.Tx) error) error

// SqlDB returns the underlying *sql.DB for use by package stores.
func (db *DB) SqlDB() *sql.DB
```

## Package store types

Each package gets a store type that accepts the db handle:

```go
// workspace/store.go
type Store struct { db *sql.DB }
func NewStore(db *sql.DB) *Store

// agent/store.go  (already exists, will be refactored)
type Store struct { db *sql.DB; eventsDir string; ... }
func NewStore(db *sql.DB, opts Options) *Store

// job/store.go  (currently manager.go, will be refactored)
type Manager struct { db *sql.DB; ... }
func Open(db *sql.DB, opts OpenOptions) *Manager
```

The CLI layer constructs `internal/db.DB` once and passes it to each package's
constructor.

## JSON-to-SQLite migration

On `Open()`, when the database file doesn't exist:

1. Check if `LegacyJSONPath` is set and the file exists.
2. If so, prompt: "Incrementum needs to migrate state from JSON to SQLite.
   Continue? [Y/n]" (skipped if `SkipConfirm` is true).
3. Create the database and run all migrations.
4. Parse `state.json` and INSERT all data (all repos) into the new tables.
5. Rename `state.json` to `state.json.bak`.

## Implementation steps

Each step is a single commit with passing tests.

### 1. Add `internal/db` package with migration runner

- Add `modernc.org/sqlite` dependency.
- Create `internal/db/db.go` with `Open()`, `Close()`, `Tx()`, pragma setup.
- Create `internal/db/migrate.go` with `schema_version` table and migration
  runner.
- Create `internal/db/migrations/001_initial.sql` with the full schema above.
- Add tests: open a db, verify tables exist, verify idempotent migration,
  verify pragmas.

### 2. Add `internal/db` JSON migration

- Add `internal/db/legacy.go` with JSON-to-SQLite import logic.
- Wire it into `Open()`: detect missing db + existing JSON, prompt, import.
- Add tests: create a `state.json` fixture, run Open(), verify data in
  SQLite, verify `state.json.bak` created.

### 3. Migrate workspace package to SQLite

- Move `WorkspaceInfo` and `WorkspaceStatus` types from `internal/state` into
  `workspace`.
- Add a `workspace.Store` (or refactor `Pool`) to use `*sql.DB` instead of
  `*state.Store`.
- Replace all `stateStore.Update()` calls with SQL queries/transactions.
- Update `workspace` tests.
- Update CLI layer (`cmd/ii/workspace.go`) to construct `internal/db.DB` and
  pass it to the workspace package.
- Remove workspace-related code from `internal/state/types.go`.

### 4. Migrate agent package to SQLite

- Move `AgentSession` and `AgentSessionStatus` types from `internal/state`
  into `agent`.
- Refactor `agent.Store` to use `*sql.DB` instead of `*state.Store`.
- Replace all `stateStore.Update()` calls with SQL queries.
- Update `agent` tests.
- Update CLI layer (`cmd/ii/agent_helpers.go`).
- Remove agent-session-related code from `internal/state/types.go`.

### 5. Migrate job package to SQLite

- Move `Job`, `JobChange`, `JobCommit`, `JobReview`, `JobAgentSession`, and
  all associated status/stage types from `internal/state` into `job`.
- Refactor `job.Manager` to use `*sql.DB` instead of `*state.Store`.
- Replace all `stateStore.Update()` calls with SQL queries/transactions.
- Derived methods like `CurrentChange()` and `CurrentCommit()` become SQL
  queries or in-memory computation after loading.
- Update `job` tests.
- Update CLI layer (`cmd/ii/job.go`).
- Remove job-related code from `internal/state/types.go`.

### 6. Remove `internal/state` and lock file

- Delete `internal/state/` entirely.
- Remove `state.lock` creation/usage.
- Move `SanitizeRepoName`, `GetOrCreateRepoName`, and path normalization
  helpers into `internal/db` (they operate on the `repos` table now).
- Update `internal/paths` to add `DefaultDBPath()` returning
  `~/.local/state/incrementum/state.db`.
- Clean up any remaining references.

### 7. Update specs

- Update `internal-state.md` → replace with `internal-db.md`.
- Update `workspace.md`, `agent.md`, `job.md`, `job-changes.md` to reflect
  SQLite storage.
- Remove backward-compat notes from `internal-state.md`.
- Update the spec table in `specs/README.md`.
