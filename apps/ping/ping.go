package main

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"monks.co/pkg/beeminder"
	"monks.co/pkg/dbserver"
	"monks.co/pkg/util"
)

var (
	//go:embed templates/*
	files     embed.FS
	templates map[string]*template.Template
)

func init() {
	ts, err := util.ReadTemplates(files, "templates")
	if err != nil {
		panic(err)
	}
	templates = ts
}

type server struct {
	*dbserver.DBServer
}

func New() *server {
	s := &server{
		dbserver.New("ping", migrate),
	}

	s.HandleFunc("/", s.ListPeople)
	s.HandleFunc("/person/", s.ShowPerson)
	s.HandleFunc("/commands/bump", s.Bump)
	s.HandleFunc("/commands/ping-person", s.PingPerson)
	s.HandleFunc("/commands/add-person", s.AddPerson)
	s.HandleFunc("/commands/update-person", s.UpdatePerson)

	return s
}

func migrate(conn *sqlite.Conn) error {
	if err := sqlitex.ExecScript(conn, `
		create table if not exists people (
			slug text primary key not null
		);

		create table if not exists pings (
			person_slug text not null,
			at_ms integer not null,

			notes text
		);`); err != nil {
		return err
	}

	const schemaQuery = `select sql from sqlite_master where tbl_name = 'people' and type = 'table';`
	var hasIsActive bool
	onResult := func(stmt *sqlite.Stmt) error {
		txt := stmt.ColumnText(0)
		hasIsActive = strings.Contains(txt, "is_active")
		return nil
	}
	if err := sqlitex.Exec(conn, schemaQuery, onResult); err != nil {
		return err
	}

	if !hasIsActive {
		if err := sqlitex.ExecScript(conn, `alter table people add column is_active int default 1;`); err != nil {
			return err
		}
	}

	return nil
}

func (s *server) ListPeople(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	people, err := listPeople(conn)
	if err != nil {
		s.InternalServerError(w, req, err)
		return
	}

	if err := templates["list.gohtml"].Execute(w, people); err != nil {
		s.InternalServerError(w, req, err)
		return
	}
}

func (s *server) ShowPerson(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	slug := req.URL.Query().Get("slug")

	person, pings, err := showPerson(conn, slug)
	if err != nil {
		s.InternalServerError(w, req, err)
		return
	}

	data := struct {
		Slug              string
		IsActive          bool
		IsLongestUnpinged bool
		Pings             []Ping
	}{
		Slug:              slug,
		IsActive:          person.IsActive,
		IsLongestUnpinged: person.IsLongestUnpinged,
		Pings:             pings,
	}

	if err := templates["show.gohtml"].Execute(w, data); err != nil {
		s.InternalServerError(w, req, err)
		return
	}
}

func (s *server) AddPerson(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		s.Error(w, req, http.StatusBadRequest, err)
		return
	}

	slug := req.Form.Get("slug")
	notes := req.Form.Get("notes")

	if err := addPerson(conn, slug); err != nil {
		s.InternalServerError(w, req, err)
		return
	}

	if notes != "" {
		if err := addPing(conn, slug, notes); err != nil {
			s.InternalServerError(w, req, err)
			return
		}
	}

	http.Redirect(w, req, "/ping", 302)
}

func (s *server) UpdatePerson(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		s.Error(w, req, http.StatusBadRequest, err)
		return
	}

	slug := req.Form.Get("slug")
	isActive := req.Form.Get("is_active") == "on"

	if err := updatePerson(conn, slug, isActive); err != nil {
		s.InternalServerError(w, req, err)
		return
	}

	http.Redirect(w, req, fmt.Sprintf("/ping/person?slug=%s", slug), 302)
}

func (s *server) Bump(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := beeminder.Insert(beeminder.Datapoint{User: "ajm", Goal: "ping", Value: 1, Comment: "bump"}); err != nil {
		s.InternalServerError(w, req, err)
		return
	}

	http.Redirect(w, req, "/ping", 302)
}

func (s *server) PingPerson(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		s.Error(w, req, http.StatusBadRequest, err)
		return
	}

	slug := req.Form.Get("slug")
	notes := req.Form.Get("notes")

	person, _, err := showPerson(conn, slug)
	if err != nil {
		s.InternalServerError(w, req, err)
		return
	}

	if person.IsLongestUnpinged {
		if err := beeminder.Insert(beeminder.Datapoint{User: "ajm", Goal: "ping", Value: 1, Comment: slug}); err != nil {
			s.InternalServerError(w, req, err)
			return
		}
	}

	if err := addPing(conn, slug, notes); err != nil {
		s.InternalServerError(w, req, err)
		return
	}

	http.Redirect(w, req, fmt.Sprintf("/ping/person?slug=%s", slug), 302)
}
