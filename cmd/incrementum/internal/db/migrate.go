package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"monks.co/pkg/migrate"
)

//go:embed migrations/*.sql
var migrations embed.FS

func runMigrations(sqlDB *sql.DB) error {
	// Drop the legacy schema_version table from the old migration system.
	// This must happen before pkg/migrate runs so it doesn't count as a
	// user table when detecting existing databases.
	if _, err := sqlDB.Exec(`DROP TABLE IF EXISTS schema_version`); err != nil {
		return fmt.Errorf("drop legacy schema_version: %w", err)
	}
	return migrate.Run(context.Background(), migrate.Config{
		DB: sqlDB, FS: migrations, Dir: "migrations",
		Baseline: []string{"001_initial.sql"},
	})
}
