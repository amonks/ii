package db

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var ErrCollision = fmt.Errorf("collision")

type DB struct {
	path string

	wMu sync.Mutex
	db  *gorm.DB
}

func New(path string) *DB {
	return &DB{
		path: path,
	}
}

func (db *DB) Start() error {
	gormdb, err := gorm.Open(sqlite.Open(db.path), &gorm.Config{
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
		return fmt.Errorf("failed to open database: %w", err)
	}

	if tx := gormdb.Exec(`PRAGMA journal_mode=wal;`); tx.Error != nil {
		return tx.Error
	}

	db.db = gormdb
	return nil
}

func (db *DB) Stop() error {
	sqldb, err := db.db.DB()
	if err != nil {
		return fmt.Errorf("error accessing database connection: %w", err)
	}
	if err := sqldb.Close(); err != nil {
		return fmt.Errorf("error closing connection: %w", err)
	}
	return nil
}
