# pkg/migrate

SQLite migration runner with version tracking, content-addressed drift detection, and baseline support.

## API

```go
package migrate

type Config struct {
    DB       *sql.DB
    FS       fs.FS
    Dir      string   // directory within FS, e.g. "migrations"
    Baseline []string // filenames to record-but-not-execute on existing databases
}

func Run(ctx context.Context, cfg Config) error
```

Accepts `*sql.DB` directly so it works with any driver (GORM apps extract it via `db.DB.DB()`, others use modernc or mattn).

## Tracking Table

```sql
CREATE TABLE applied_migrations (
    migration_filename TEXT NOT NULL PRIMARY KEY,
    applied_at TEXT NOT NULL,
    ddl TEXT NOT NULL
);
```

## Behavior

1. If `applied_migrations` doesn't exist:
   - Check `sqlite_master` for other user tables
   - If count > 0 (existing database): create `applied_migrations`, record each `Baseline` file as applied **without executing**, then proceed to step 2 for remaining files
   - If count == 0 (fresh database): create `applied_migrations`, proceed to step 2 for all files (baseline files execute normally)
2. Load applied set from `applied_migrations`
3. Walk `.sql` files from FS sorted lexicographically
4. For each file:
   - Already applied + same content: skip
   - Already applied + different content: error ("migration has diverged")
   - Not applied: execute in a transaction, record in `applied_migrations`
5. If `applied_migrations` contains filenames not on disk: error ("deleted migration")

Non-SQL files are ignored. There are no down migrations.

## Baseline Support

Baseline files let existing production databases adopt `pkg/migrate` without re-executing their initial schema. On a fresh database, baseline files execute normally. On an existing database (one with user tables but no `applied_migrations`), baseline files are recorded as applied without execution.

## Integration with pkg/database

`pkg/database` provides `MigrateFS`:

```go
func (db *DB) MigrateFS(ctx context.Context, fsys fs.FS, dir string, baseline ...string) error
```

This extracts the `*sql.DB` from GORM and delegates to `migrate.Run`.

## Migration File Conventions

- Files must be in `.sql` format
- Files are applied in lexicographic order by filename
- Convention: `NNN_description.sql` (e.g., `001_baseline.sql`, `002_add_column.sql`)
- Once applied, a migration's content must never change (drift detection will error)
- Once applied, a migration must never be deleted (deleted migration detection will error)
