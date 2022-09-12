package golink

import (
	"context"
	"time"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

type Shortening struct {
	URL       string
	Key       string
	CreatedAt *time.Time
}

func List(ctx context.Context, conn *sqlite.Conn) ([]Shortening, error) {
	const query = `select key, url, created_at from urls order by created_at desc`

	var ss []Shortening
	onResult := func (stmt *sqlite.Stmt) error {
		s := Shortening{
			Key: stmt.ColumnText(0),
			URL: stmt.ColumnText(1),
		}
		createdAt, err := time.Parse("2006-09-02 15:04:05", stmt.ColumnText(2))
		if err != nil {
			return err
		}
		s.CreatedAt = &createdAt
		ss = append(ss, s)
		return nil
	}

	if err := sqlitex.Exec(conn, query, onResult); err != nil {
		return nil, err
	}

	return ss, nil
}

func Get(ctx context.Context, conn *sqlite.Conn, key string) (string, error) {
	const query = `select url from urls where key = ?`

	var url string
	onResult := func(stmt *sqlite.Stmt) error {
		url = stmt.ColumnText(0)
		return nil
	}

	if err := sqlitex.Exec(conn, query, onResult, key); err != nil {
		return url, err
	}

	return url, nil
}

func Set(ctx context.Context, conn *sqlite.Conn, key, url string) error {
	const query = `insert into urls (key, url, created_at) values (?, ?, current_timestamp)`

	if err := sqlitex.Exec(conn, query, nil, key, url); err != nil {
		return err
	}

	return nil
}

func Delete(ctx context.Context, conn *sqlite.Conn, key string) error {
	const query = `delete from urls where key = ?`

	if err := sqlitex.Exec(conn, query, nil, key); err != nil {
		return err
	}

	return nil
}
