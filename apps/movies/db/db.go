package db

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	//go:embed schema.sql
	schema string

	//go:embed schema_tv.sql
	tvSchema string

	//go:embed migrate_tv.sql
	migrateTVSchema string

	//go:embed migrate_episode_files.sql
	migrateEpisodeFilesSchema string

	//go:embed migrate_season_status.sql
	migrateSeasonStatusSchema string

	//go:embed migrate_ignore_type.sql
	migrateIgnoreTypeSchema string
)

var ErrCollision = fmt.Errorf("collision")

type DB struct {
	*gorm.DB

	path string

	mutex               sync.Mutex
	subscriptions       []chan *Movie
	tvSubscriptions     []chan *TVSeason
	movieStubSubscriptions []chan *Stub
	tvStubSubscriptions    []chan *Stub

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

func (db *DB) SubscribeTV() chan *TVSeason {
	if db.parent != nil {
		return db.parent.SubscribeTV()
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	c := make(chan *TVSeason)
	db.tvSubscriptions = append(db.tvSubscriptions, c)
	return c
}

func (db *DB) SubscribeMovieStub() chan *Stub {
	if db.parent != nil {
		return db.parent.SubscribeMovieStub()
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	c := make(chan *Stub)
	db.movieStubSubscriptions = append(db.movieStubSubscriptions, c)
	return c
}

func (db *DB) SubscribeTVStub() chan *Stub {
	if db.parent != nil {
		return db.parent.SubscribeTVStub()
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	c := make(chan *Stub)
	db.tvStubSubscriptions = append(db.tvStubSubscriptions, c)
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

func (db *DB) notifyTV(s *TVSeason) {
	if db.parent != nil {
		db.parent.notifyTV(s)
		return
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	for _, c := range db.tvSubscriptions {
		c <- s
		close(c)
	}

	db.tvSubscriptions = nil
}

func (db *DB) notifyMovieStub(s *Stub) {
	if db.parent != nil {
		db.parent.notifyMovieStub(s)
		return
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	for _, c := range db.movieStubSubscriptions {
		c <- s
		close(c)
	}

	db.movieStubSubscriptions = nil
}

func (db *DB) notifyTVStub(s *Stub) {
	if db.parent != nil {
		db.parent.notifyTVStub(s)
		return
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	for _, c := range db.tvStubSubscriptions {
		c <- s
		close(c)
	}

	db.tvStubSubscriptions = nil
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

	db.DB = gormdb

	if err := gormdb.Exec(schema).Error; err != nil {
		return fmt.Errorf("error migrating movie schema: %w", err)
	}

	if err := gormdb.Exec(tvSchema).Error; err != nil {
		return fmt.Errorf("error migrating TV schema: %w", err)
	}

	// Run TV migration scripts (safe to run multiple times)
	if err := gormdb.Exec(migrateTVSchema).Error; err != nil {
		// Ignore duplicate column errors - this is expected if the column already exists
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("error running TV migration: %w", err)
		}
	}

	// Run episode_files migration script (safe to run multiple times)
	if err := gormdb.Exec(migrateEpisodeFilesSchema).Error; err != nil {
		// Ignore duplicate column errors - this is expected if the column already exists
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("error running episode_files migration: %w", err)
		}
	}

	// Run season_status migration script (safe to run multiple times)
	if err := gormdb.Exec(migrateSeasonStatusSchema).Error; err != nil {
		// Ignore duplicate column errors - this is expected if the column already exists
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("error running season_status migration: %w", err)
		}
	}

	// Run ignore_type migration script (safe to run multiple times)
	if err := gormdb.Exec(migrateIgnoreTypeSchema).Error; err != nil {
		// Ignore duplicate column errors - this is expected if the column already exists
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("error running ignore_type migration: %w", err)
		}
	}

	if err := db.PopulateMovieWatches(); err != nil {
		return err
	}

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
