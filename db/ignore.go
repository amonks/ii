package db

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

type Ignore struct {
	ImportedFromPath string
}

func (db *DB) PathIsIgnored(path string) (bool, error) {
	c, err := db.conn()
	defer c.release()
	if err != nil {
		return false, err
	}

	var ignore bool
	const q = `select true from ignores where path = ?;`
	f := func(_ *sqlite.Stmt) error {
		ignore = true
		return nil
	}
	if err := sqlitex.Exec(c.Conn, q, f, path); err != nil {
		return false, err
	}

	return ignore, nil
}

func (db *DB) IgnorePath(path string) error {
	c, err := db.conn()
	defer c.release()
	if err != nil {
		return err
	}

	const q = `insert into ignores (path) values (?);`
	if err := sqlitex.Exec(c.Conn, q, nil, path); err != nil {
		return err
	}

	return nil
}


