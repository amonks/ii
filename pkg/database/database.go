package database

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"monks.co/pkg/env"
	"monks.co/pkg/migrate"
	"monks.co/pkg/tailnet"
)

type DB struct {
	*gorm.DB
	replication *Replication
}

func OpenFromDataFolder(name string) (*DB, error) {
	path := env.InMonksData(name + ".db")
	return Open(path)
}

func Open(path string) (*DB, error) {
	// Start litestream replication before opening GORM. Litestream needs
	// to set up WAL monitoring first. Skip for :memory: databases (tests)
	// and when the tailnet isn't ready (tests that use file-backed DBs).
	var repl *Replication
	if path != ":memory:" && tailnetReady() {
		var err error
		repl, err = StartReplication(context.Background(), path)
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
		if repl != nil {
			repl.Close()
		}
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}

	// SQLite only supports one concurrent writer. Limit the connection
	// pool to a single connection to avoid SQLITE_BUSY contention between
	// pooled connections from concurrent goroutines.
	sqlDB, err := db.DB()
	if err != nil {
		if repl != nil {
			repl.Close()
		}
		return nil, fmt.Errorf("getting sql.DB for %s: %w", path, err)
	}
	sqlDB.SetMaxOpenConns(1)

	// Ensure WAL writes are fsynced immediately. The default
	// synchronous=FULL should already do this, but set it explicitly
	// so durability doesn't depend on compile-time defaults.
	if _, err := sqlDB.Exec("PRAGMA synchronous = FULL"); err != nil {
		if repl != nil {
			repl.Close()
		}
		return nil, fmt.Errorf("setting synchronous mode for %s: %w", path, err)
	}

	return &DB{db, repl}, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}

	// Checkpoint the WAL to flush all data to the main database file
	// before closing. This ensures durability across machine restarts
	// where the WAL file might not survive.
	if _, err := sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		slog.Warn("wal checkpoint on close failed", "error", err)
	}

	if err := sqlDB.Close(); err != nil {
		return err
	}
	if db.replication != nil {
		if err := db.replication.Close(); err != nil {
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

