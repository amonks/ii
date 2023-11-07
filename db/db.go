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
	*gorm.DB

	path string

	mutex         sync.Mutex
	subscriptions []chan *Movie
}

func (db *DB) Subscribe() chan *Movie {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	c := make(chan *Movie)
	db.subscriptions = append(db.subscriptions, c)
	return c
}

func (db *DB) notify(m *Movie) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	for _, c := range db.subscriptions {
		c <- m
		close(c)
	}

	db.subscriptions = nil
}

func (db *DB) close() {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	for _, c := range db.subscriptions {
		close(c)
	}
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
		return fmt.Errorf("error activating WAL: %w", tx.Error)
	}

	if err := gormdb.AutoMigrate(&Movie{}, &Ignore{}, &Stub{}); err != nil {
		return fmt.Errorf("migration error: %w", err)
	}

	db.DB = gormdb
	return nil
}

func (db *DB) Stop() error {
	sqldb, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("error accessing database connection: %w", err)
	}
	if err := sqldb.Close(); err != nil {
		return fmt.Errorf("error closing connection: %w", err)
	}
	return nil
}
