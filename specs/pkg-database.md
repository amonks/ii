# Database Package

## Overview

Wraps GORM + SQLite to provide a shared database-opening utility with
WAL mode, busy timeout, and migration support.

Code: [pkg/database/](../pkg/database/)

## Opening a Database

- `Open(path string) (*DB, error)` — opens SQLite at the given path with
  WAL mode and a 5-second busy timeout.
- `OpenFromDataFolder(name string) (*DB, error)` — opens
  `$MONKS_DATA/<name>.db`.

The returned `DB` embeds `*gorm.DB` so all GORM methods are available.

## Migrations

Two migration strategies are used across the codebase:

### Embedded SQL Migrations

```go
migrations, _ := database.LoadMigrationsFromFS(migrationsFS, "migrations")
db.Migrate(migrations)
```

`LoadMigrationsFromFS` loads and sorts `.sql` files from an `embed.FS`.
`Migrate` applies them in order, tracking applied migrations and tolerating
duplicate-column `ALTER TABLE` errors (best-effort schema evolution).

### GORM AutoMigrate

Simpler apps use `db.AutoMigrate(&Model{})` directly.

## Dependencies

- `pkg/env` — for `$MONKS_DATA` path resolution
- `gorm.io/gorm` + `gorm.io/driver/sqlite`
