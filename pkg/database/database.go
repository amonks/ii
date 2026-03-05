package database

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/benbjohnson/litestream"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"monks.co/pkg/env"
	"monks.co/pkg/tailnet"
)

type DB struct {
	*gorm.DB
	litestreamStore *litestream.Store
}

func OpenFromDataFolder(name string) (*DB, error) {
	path := env.InMonksData(name + ".db")
	return Open(path)
}

func Open(path string) (*DB, error) {
	// Start litestream replication before opening GORM. Litestream needs
	// to set up WAL monitoring first. Skip for :memory: databases (tests)
	// and when the tailnet isn't ready (tests that use file-backed DBs).
	var store *litestream.Store
	if path != ":memory:" && tailnetReady() {
		var err error
		store, err = startReplication(context.Background(), path)
		if err != nil {
			return nil, fmt.Errorf("litestream replication for %s: %w", path, err)
		}
	}

	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
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
		if store != nil {
			store.Close(context.Background())
		}
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	return &DB{db, store}, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}
	if db.litestreamStore != nil {
		if err := db.litestreamStore.Close(context.Background()); err != nil {
			return err
		}
	}
	return nil
}

func tailnetReady() bool {
	select {
	case <-tailnet.ReadyChan():
		return true
	default:
		return false
	}
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
			// Ignore idempotency errors from re-running migrations
			if strings.Contains(err.Error(), "duplicate column name") {
				log.Printf("Migration %s: column already exists, skipping", migration.ID)
				continue
			}
			if strings.Contains(err.Error(), "already exists") {
				log.Printf("Migration %s: already exists, skipping", migration.ID)
				continue
			}
			return fmt.Errorf("migration %s failed: %w", migration.ID, err)
		}
		log.Printf("Applied migration: %s", migration.ID)
	}
	return nil
}
