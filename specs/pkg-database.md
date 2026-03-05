# Database Package

## Overview

Wraps GORM + SQLite to provide a shared database-opening utility with
WAL mode, busy timeout, migration support, and automatic litestream
replication to the vault.

Code: [pkg/database/](../pkg/database/)

## Opening a Database

- `Open(path string) (*DB, error)` — opens SQLite at the given path with
  WAL mode and a 5-second busy timeout. Starts litestream replication
  if the tailnet is ready and the path is not `:memory:`.
- `OpenFromDataFolder(name string) (*DB, error)` — opens
  `$MONKS_DATA/<name>.db`.

The returned `DB` embeds `*gorm.DB` so all GORM methods are available.

### Startup Order

Apps **must** call `tailnet.WaitReady(ctx)` before opening any database.
This ensures the tailnet is available for litestream to connect to the
vault SFTP server. If `WaitReady` has not been called (e.g., in tests),
replication is skipped.

## Litestream Replication

For non-`:memory:` databases when the tailnet is ready, `Open` starts a
litestream `Store` that continuously replicates WAL changes to
`monks-vault-thor` over SFTP. The SFTP connection is made through tsnet
(not standard `net.Dial`) so it works on Fly.io where tailnet hosts are
only reachable via the userspace networking stack.

The vault server roots each peer's SFTP session at a directory named
after their tailscale machine name (determined via WhoIs). The client
only specifies the database filename.

`Close()` shuts down the litestream store before closing the GORM
connection.

## Migrations

All apps use `pkg/migrate` via the `MigrateFS` convenience method:

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS

db.MigrateFS(ctx, migrationsFS, "migrations", "001_baseline.sql")
```

`MigrateFS` extracts the underlying `*sql.DB` and delegates to
`migrate.Run`. Baseline filenames are recorded without execution on
existing databases. See [pkg-migrate.md](pkg-migrate.md) for details.

## Dependencies

- `pkg/env` — for `$MONKS_DATA` path resolution
- `pkg/tailnet` — for tsnet dialing and readiness checks
- `gorm.io/gorm` + `gorm.io/driver/sqlite`
- `github.com/benbjohnson/litestream` — WAL replication engine
- `github.com/pkg/sftp` + `golang.org/x/crypto/ssh` — SFTP client
