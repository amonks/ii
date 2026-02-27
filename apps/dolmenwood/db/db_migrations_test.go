package db

import (
	"database/sql"
	"strings"
	"testing"

	"monks.co/pkg/database"
)

func TestCompanionStatMigrationIgnoresExistingColumns(t *testing.T) {
	d, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := d.Exec(`
		CREATE TABLE companions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			character_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			breed TEXT NOT NULL DEFAULT '',
			hp_current INTEGER NOT NULL DEFAULT 0,
			hp_max INTEGER NOT NULL DEFAULT 0,
			ac INTEGER NOT NULL DEFAULT 10
		);
	`).Error; err != nil {
		t.Fatalf("create companions: %v", err)
	}
	for _, stmt := range migrationCompanionStats {
		d.Exec(stmt)
	}

	if err := d.Error; err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			t.Fatalf("migration error: %v", err)
		}
	}

	type columnInfo struct {
		Name      string         `gorm:"column:name"`
		DfltValue sql.NullString `gorm:"column:dflt_value"`
	}

	var columns []columnInfo
	if err := d.Raw("PRAGMA table_info(companions)").Scan(&columns).Error; err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}

	defaults := map[string]sql.NullString{}
	for _, col := range columns {
		defaults[col.Name] = col.DfltValue
	}

	checks := map[string]string{
		"ac":            "10",
		"speed":         "40",
		"load_capacity": "0",
		"level":         "1",
		"attack":        "''",
		"morale":        "0",
	}

	for column, expected := range checks {
		value, ok := defaults[column]
		if !ok {
			t.Fatalf("expected %s column in companions", column)
		}
		if !value.Valid || value.String != expected {
			t.Errorf("%s default = %q, want %q", column, value.String, expected)
		}
	}
}
