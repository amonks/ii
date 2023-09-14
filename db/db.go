package db

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

var ErrCollision = fmt.Errorf("collision")

type DB struct {
	path string
	pool *sqlitex.Pool

	wMu sync.Mutex
}

func New(path string) *DB {
	return &DB{
		path: path,
	}
}

type conn struct {
	*sqlite.Conn
	db *DB
}

func (conn *conn) release() {
	conn.db.pool.Put(conn.Conn)
	conn.db.wMu.Unlock()
}

func (db *DB) conn() (*conn, error) {
	db.wMu.Lock()

	if db.pool == nil {
		return nil, errors.New("db not started")
	}

	c := db.pool.Get(context.Background())
	if c == nil {
		return nil, errors.New("could not get connection")
	}

	return &conn{Conn: c, db: db}, nil
}

func (db *DB) Start() error {
	pool, err := sqlitex.Open(fmt.Sprintf("file:%s", db.path), 0, 10)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	db.pool = pool

	c, err := db.conn()
	defer c.release()
	if err != nil {
		return err
	}

	if err := sqlitex.Exec(c.Conn, `PRAGMA journal_mode=wal;`, nil); err != nil {
		return fmt.Errorf("failed to enable wal mode: %w", err)
	}

	if err := sqlitex.Exec(c.Conn, `
		create table if not exists ignores (
			path text unique
		);`, nil); err != nil {
		return fmt.Errorf("failed to create `ignores` table: %w", err)
	}

	if err := sqlitex.Exec(c.Conn, `
		create table if not exists movies (
			id                 int primary key,
			title              text,
			original_title     text,
			tagline            text,
			overview           text,
			runtime            int,
			genres             text,
			languages          text,
			release_date       text,

			extension          text,
			library_path       text unique,
			imported_from_path text unique,

			is_imported        integer,

			tmdb_json          text,
			poster_path        text,

			tmdb_credits_json  text,
			director_name      text,

			writer_name        text
		);`, nil); err != nil {
		return fmt.Errorf("failed to create `movies` table: %w", err)
	}

	return nil
}
