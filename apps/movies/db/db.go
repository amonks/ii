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

	parent *DB
}

//go:generate go run golang.org/x/tools/cmd/stringer -type MediaType
type MediaType int

const (
	mediaTypeInvalid MediaType = iota
	MediaTypeTV
	MediaTypeMovie
)

func (db *DB) Transaction(f func(*DB) error) error {
	if db.parent != nil {
		panic("Transaction called on tx")
	}

	return db.DB.Transaction(func(tx *gorm.DB) error {
		return f(&DB{
			DB:     tx,
			parent: db,
		})
	})
}

func (db *DB) Subscribe() chan *Movie {
	if db.parent != nil {
		return db.parent.Subscribe()
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	c := make(chan *Movie)
	db.subscriptions = append(db.subscriptions, c)
	return c
}

func (db *DB) notify(m *Movie) {
	if db.parent != nil {
		db.parent.notify(m)
		return
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	for _, c := range db.subscriptions {
		c <- m
		close(c)
	}

	db.subscriptions = nil
}

func (db *DB) close() {
	if db.parent != nil {
		panic("close called on tx")
	}

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
	if db.parent != nil {
		panic("Start called on tx")
	}

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

	if err := gormdb.AutoMigrate(
		&Movie{},
		&Ignore{},
		&Stub{},
		&Watch{},
		&QueuedMovie{},
	); err != nil {
		return fmt.Errorf("migration error: %w", err)
	}

	db.DB = gormdb
	return nil
}

func (db *DB) Stop() error {
	if db.parent != nil {
		panic("Stop called on tx")
	}

	sqldb, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("error accessing database connection: %w", err)
	}
	if err := sqldb.Close(); err != nil {
		return fmt.Errorf("error closing connection: %w", err)
	}
	return nil
}
