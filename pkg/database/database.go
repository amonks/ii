package database

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"monks.co/pkg/env"
)

type DB struct {
	*gorm.DB
}

func OpenFromDataFolder(name string) (*DB, error) {
	path := env.InMonksData(name + ".db")
	return Open(path)
}

func Open(path string) (*DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.New(
			log.New(os.Stderr, "\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Warn,
				IgnoreRecordNotFoundError: true,
				ParameterizedQueries:      true,
				Colorful:                  true,
			},
		),
	})
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	if err := db.Exec("pragma journal_mode='wal';").Error; err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}
	return nil
}

// Migration represents a database migration
type Migration struct {
	ID   string
	SQL  string
}

// LoadMigrationsFromFS loads migrations from an embedded filesystem
func LoadMigrationsFromFS(fs embed.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var migrations []Migration
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := fs.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}

		// Use filename without extension as ID
		id := strings.TrimSuffix(entry.Name(), ".sql")
		migrations = append(migrations, Migration{
			ID:  id,
			SQL: string(content),
		})
	}

	// Sort migrations by filename to ensure proper order
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].ID < migrations[j].ID
	})

	return migrations, nil
}

// Migrate runs a sequence of migrations
func (db *DB) Migrate(migrations []Migration) error {
	for _, migration := range migrations {
		if err := db.Exec(migration.SQL).Error; err != nil {
			// Ignore "duplicate column name" errors
			if strings.Contains(err.Error(), "duplicate column name") {
				log.Printf("Migration %s: column already exists, skipping", migration.ID)
				continue
			}
			return fmt.Errorf("migration %s failed: %w", migration.ID, err)
		}
		log.Printf("Applied migration: %s", migration.ID)
	}
	return nil
}
