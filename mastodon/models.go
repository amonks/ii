package mastodon

import (
	"monks.co/logger"
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

type Post struct {
	ID   string
	JSON []byte
}

type model struct {
	logger.Logger
}

func NewModel() *model {
	return &model{
		Logger: logger.New("mastodon model"),
	}
}

func (m *model) migrate(conn *sqlite.Conn) error {
	if err := sqlitex.ExecScript(conn, `
		create table if not exists posts (
			id text primary key not null,
			json text
		);`); err != nil {
		return err
	}
	return nil
}

func (m *model) listPosts(conn *sqlite.Conn) ([]Post, error) {
	const query = `
		select id, json
		from posts;`

	posts := []Post{}
	onResult := func(stmt *sqlite.Stmt) error {
		post := Post{
			ID:   stmt.ColumnText(0),
			JSON: []byte(stmt.ColumnText(1)),
		}
		posts = append(posts, post)
		return nil
	}
	if err := sqlitex.Exec(conn, query, onResult); err != nil {
		return nil, err
	}

	return posts, nil
}
