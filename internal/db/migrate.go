package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrations embed.FS

const schemaTable = "schema_version"

func migrate(db *sql.DB) error {
	if err := ensureSchemaTable(db); err != nil {
		return err
	}

	currentVersion, err := currentSchemaVersion(db)
	if err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	for _, name := range files {
		version, err := migrationVersion(name)
		if err != nil {
			return err
		}
		if version <= currentVersion {
			continue
		}
		contents, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := applyMigration(db, version, string(contents)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		currentVersion = version
	}

	return nil
}

func ensureSchemaTable(db *sql.DB) error {
	_, err := db.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (version INTEGER NOT NULL);`, schemaTable))
	if err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	row := db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s;`, schemaTable))
	var count int
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("count schema_version: %w", err)
	}
	if count != 0 {
		return nil
	}

	_, err = db.Exec(fmt.Sprintf(`INSERT INTO %s (version) VALUES (0);`, schemaTable))
	if err != nil {
		return fmt.Errorf("initialize schema_version: %w", err)
	}
	return nil
}

func currentSchemaVersion(db *sql.DB) (int, error) {
	row := db.QueryRow(fmt.Sprintf(`SELECT version FROM %s LIMIT 1;`, schemaTable))
	var version int
	if err := row.Scan(&version); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

func applyMigration(db *sql.DB, version int, sqlText string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}

	if _, err := tx.Exec(sqlText); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec migration: %w", err)
	}

	if _, err := tx.Exec(fmt.Sprintf(`UPDATE %s SET version = ?;`, schemaTable), version); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update schema version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}

	return nil
}

func migrationVersion(name string) (int, error) {
	if !strings.HasSuffix(name, ".sql") {
		return 0, fmt.Errorf("migration %q must have .sql extension", name)
	}
	parts := strings.SplitN(name, "_", 2)
	if len(parts) < 2 {
		return 0, fmt.Errorf("migration %q missing version prefix", name)
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("migration %q invalid version: %w", name, err)
	}
	return version, nil
}
