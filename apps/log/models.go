package log

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"monks.co/pkg/logger"
)

type Log struct {
	IsPublished  bool
	HappenedAtMS int64
	Subject      string
	Content      string
}

type model struct {
	logger.Logger
}

func NewModel() *model {
	return &model{
		Logger: logger.New("log model"),
	}
}

func (m *model) migrate(conn *sqlite.Conn) error {
	return nil
}

func (m *model) logs(conn *sqlite.Conn) ([]Log, error) {
	logs := []Log{}
	const query = `
		select is_published, happened_at_ms, subject, content
		from log
		order by happened_at_ms desc;`

	onResult := func(stmt *sqlite.Stmt) error {
		logs = append(logs, Log{
			IsPublished:  stmt.ColumnInt(0) == 1,
			HappenedAtMS: stmt.ColumnInt64(1),
			Subject:      stmt.ColumnText(2),
			Content:      stmt.ColumnText(3),
		})
		return nil
	}
	if err := sqlitex.Exec(conn, query, onResult); err != nil {
		return nil, err
	}

	return logs, nil
}

func (m *model) addLog(conn *sqlite.Conn, log Log) error {
	const query = `
		insert into logs
			(
				is_published,
				happened_at_ms,
				subject,
				content
			)
		values
			(
				?,
				?,
				?,
				?
			);`
	err := sqlitex.Exec(conn, query, nil, log.IsPublished, log.HappenedAtMS, log.Subject, log.Content)
	return err
}
