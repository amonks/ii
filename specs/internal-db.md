# Internal DB

## Overview
The `internal/db` package owns the SQLite connection for incrementum state. It opens the single state database, applies migrations, and configures SQLite pragmas.

## Responsibilities
- Open the SQLite database at the configured path.
- Apply embedded SQL migrations in version order.
- Maintain the `schema_version` table.
- Configure recommended pragmas (WAL, busy timeout, foreign keys, etc.).

## API
- `Open(path, opts)` returns a `*DB` wrapper with the configured connection.
- `DB.Close()` closes the connection.
- `DB.Tx(fn)` runs a callback in a transaction.
- `DB.SqlDB()` exposes the underlying `*sql.DB` for domain stores.

## Migrations
Migration files live in `internal/db/migrations` and are embedded. Files are named with a numeric prefix (`001_*.sql`, `002_*.sql`, ...). The `schema_version` table tracks the current version.

## Pragmas
On open, the database is configured with:

```sql
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -2000;
```
