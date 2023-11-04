package database

import (
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type DB struct {
	*gorm.DB
}

func Open(path string) (*DB, error) {
	dbPath := filepath.Join(os.Getenv("MONKS_DATA"), path)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
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
