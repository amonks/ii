package promises

import (
	"log"
	"net/http"

	"co.monks.monks.co/dbserver"
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

func Server() *dbserver.DBServer {
	p := &promises{}

	s := dbserver.New("promises")
	s.HandleFunc("/", p.ServeHTTP)
	s.Start(p.migrate)

	return s
}

type promises struct{}

type promise struct {
	slug          string
	note          string
	createdAtMS   int64
	deadlineMS    int64
	fulfilledAtMS int64
	clickCount    int
	isVoid        bool
}

func (p *promises) migrate(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
		create table if not exists promises (
			slug text primary key not null,
			note text,
			created_at_ms integer,
			deadline_ms integer,
			fulfilled_at_ms integer,
			click_count integer,
			is_void integer
		);`)
}

func (*promises) ServeHTTP(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path

	var p promise
	const query = `select note, created_at_ms, deadline_ms, fulfilled_at_ms, click_count, is_void from promises where slug = ?`
	onResult := func(stmt *sqlite.Stmt) error {
		p = promise{
			slug:          key,
			note:          stmt.ColumnText(0),
			createdAtMS:   stmt.ColumnInt64(1),
			deadlineMS:    stmt.ColumnInt64(2),
			fulfilledAtMS: stmt.ColumnInt64(3),
			clickCount:    stmt.ColumnInt(4),
			isVoid:        stmt.ColumnInt(5) == 1,
		}
		return nil
	}

	sqlitex.Exec(conn, query, onResult, key)
	log.Println(p)
}
