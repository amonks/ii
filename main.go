package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"

	_ "github.com/mattn/go-sqlite3"

	"strings"
)

const (
	archivePath = "/data/tank/mirror/reddit/"
	dbPath      = "/data/tank/mirror/reddit/.reddit.db"
	clientID    = "-RT9cp4AERMlAEhwR01isQ"
	secret      = "mgo2f7coeJj31sIZDsdIlLZfjBfSiA"
)

type app struct {
	db *sql.DB
}

type post struct {
	Name      string `gorm:"primaryKey"`
	Title     string
	Author    string
	Subreddit string
	Url       string
	Permalink string

	Json *[]byte

	Status      string
	Filetype    *string
	Archivepath *string
}

func main() {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Println("opening db failed:", err)
		panic(err)
	}
	defer db.Close()

	c := &app{db: db}
	if err := c.migrate(); err != nil {
		fmt.Println("migrate failed:", err)
		os.Exit(1)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		n := r.URL.Query().Get("n")
		offset, _ := strconv.ParseInt(n, 10, 64)
		c.servePage(1, int(offset), w, r)
	})

	fs := http.FileServer(http.Dir("/data/tank/mirror/reddit/"))
	http.Handle("/media/", http.StripPrefix("/media/", fs))
	fmt.Println("listening on :3334")
	http.ListenAndServe(":3334", nil)
}

func (p *post) Embed() template.HTML {
	if p.Filetype == nil {
		return "no"
	}
	src := strings.Replace(*p.Archivepath, "/data/tank/mirror/reddit/", "media/", 1)
	switch *p.Filetype {
	case ".gif":
		fallthrough
	case ".jpg":
		fallthrough
	case ".png":
		tmpl, _ := template.New("gif").Parse(`<img src="{{.Src}}" />`)
		w := strings.Builder{}
		tmpl.Execute(&w, struct{ Src string }{Src: src})
		return template.HTML(w.String())
	case ".mp4":
		tmpl, _ := template.New("mp4").Parse(`<video controls autoplay loop><source src="{{ .Src}}" /></video>`)
		w := strings.Builder{}
		tmpl.Execute(&w, struct{ Src string }{Src: src})
		return template.HTML(w.String())
	default:
		return template.HTML("unexpected filetype: " + *p.Filetype)
	}
}

func (c *app) getPost(name string) (*post, error) {
	var p post
	p.Name = name
	row := c.db.QueryRow("select title, author, subreddit, url, permalink, status, filetype, archivepath from posts where name = ?", name)
	if err := row.Scan(&p.Title, &p.Author, &p.Subreddit, &p.Url, &p.Permalink, &p.Status, &p.Filetype, &p.Archivepath); err != nil {
		return nil, fmt.Errorf("error getting %s: %w", name, err)
	}
	return &p, nil
}

func (c *app) loadPostJson(p *post) error {
	var bs []byte
	row := c.db.QueryRow("select json from posts where name = ?", p.Name)
	if err := row.Scan(&bs); err != nil {
		return err
	}
	p.Json = &bs
	return nil
}

func (c *app) updatePost(p *post) error {
	if _, err := c.db.Exec("update posts set title=?, author=?, subreddit=?, url=?, permalink=?, filetype=?, status=?, archivepath=? where name = ?",
		p.Title, p.Author, p.Subreddit, p.Url, p.Permalink, p.Filetype, p.Status, p.Archivepath, p.Name); err != nil {
		return fmt.Errorf("error updating %s: %w", p.Name, err)
	}
	return nil
}

var errCollision = errors.New("Collision")

func (c *app) insertPost(p *post) error {
	if _, err := c.db.Exec("insert into posts values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);",
		p.Name, p.Title, p.Author, p.Subreddit, p.Url, p.Permalink, p.Json, p.Status, p.Filetype, p.Archivepath,
	); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: posts.name") {
			return errCollision
		}
		return fmt.Errorf("error inserting %s: %w", p.Name, err)
	}
	return nil
}

func (c *app) migrate() error {
	if _, err := c.db.Exec(`create table if not exists posts (
		name         text primary key not null,
		title        text not null,
		author       text not null,
		subreddit    text not null,
		url          text not null,
		permalink    text not null,

		json         text not null,

		status       text not null,
		filetype     text,
		archivepath  text
	);`); err != nil {
		return fmt.Errorf("migration error: %w", err)
	}
	return nil
}

var serverError = errors.New("server error")

var errUnsupportedSource = errors.New("unsupported source")
var errDeleted = errors.New("deleted")
