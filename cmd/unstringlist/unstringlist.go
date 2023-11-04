package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"monks.co/movietagger/config"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	db, err := sql.Open("sqlite3", "file:"+config.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	for {
		var id int64
		var str string
		row := db.QueryRow(`select id, languages from movies where languages like ':%' limit 1`)
		if err := row.Scan(&id, &str); err == sql.ErrNoRows {
			return nil
		} else if err != nil {
			return err
		}

		languages := split(str)
		json, err := json.Marshal(languages)
		if err != nil {
			return err
		}

		fmt.Println(id, str, "->", string(json))
		if _, err := db.Exec(`update movies set languages = ? where id = ?`, json, id); err != nil {
			return err
		}
	}
}

func join(ss []string) string {
	return ":" + strings.Join(ss, ":") + ":"
}

func split(s string) []string {
	return strings.Split(s[1:len(s)-1], ":")
}
