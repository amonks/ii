package database

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/benbjohnson/litestream"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"monks.co/pkg/env"
	"monks.co/pkg/migrate"
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

// MigrateFS runs migrations from an embedded filesystem using pkg/migrate.
func (db *DB) MigrateFS(ctx context.Context, fsys fs.FS, dir string, baseline ...string) error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return migrate.Run(ctx, migrate.Config{
		DB:       sqlDB,
		FS:       fsys,
		Dir:      dir,
		Baseline: baseline,
	})
}

