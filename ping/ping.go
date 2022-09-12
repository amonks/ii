package ping

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"co.monks.monks.co/beeminder"
	"co.monks.monks.co/dbserver"
	"co.monks.monks.co/util"
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

var (
	//go:embed templates/*
	files     embed.FS
	templates map[string]*template.Template
)

func init() {
	fmt.Println("init ping")
	ts, err := util.ReadTemplates(files, "templates")
	if err != nil {
		panic(err)
	}
	templates = ts
}

func Server() *dbserver.DBServer {
	fmt.Println("build ping server")
	s := dbserver.New("ping")
	a := &app{}
	s.HandleFunc("/ping/", a.ListPeople)
	s.HandleFunc("/ping/person/", a.ShowPerson)
	s.HandleFunc("/ping/commands/ping-person", a.PingPerson)
	s.HandleFunc("/ping/commands/add-person", a.AddPerson)
	s.HandleFunc("/ping/commands/update-person", a.UpdatePerson)
	s.Init(a.Migrate)
	fmt.Println("started server")
	return s
}

type app struct{}

func (a *app) Migrate(conn *sqlite.Conn) error {
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

func (*app) ListPeople(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	people, err := listPeople(conn)
	if err != nil {
		util.HTTPError("ping", w, req, http.StatusInternalServerError, "%s", err)
	}

	if err := templates["list.gohtml"].Execute(w, people); err != nil {
		util.HTTPError("ping", w, req, http.StatusInternalServerError, "%s", err)
		return
	}
}

func (*app) ShowPerson(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	slug := req.URL.Query().Get("slug")

	person, pings, err := showPerson(conn, slug)
	if err != nil {
		util.HTTPError("ping", w, req, http.StatusInternalServerError, "%s", err)
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
		util.HTTPError("ping", w, req, http.StatusInternalServerError, "%s", err)
		return
	}
}

func (*app) AddPerson(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		util.HTTPError("ping", w, req, http.StatusBadRequest, "%s", err)
		return
	}

	slug := req.Form.Get("slug")
	notes := req.Form.Get("notes")

	if err := addPerson(conn, slug); err != nil {
		util.HTTPError("ping", w, req, http.StatusInternalServerError, "%s", err)
		return
	}

	if notes != "" {
		if err := addPing(conn, slug, notes); err != nil {
			util.HTTPError("ping", w, req, http.StatusInternalServerError, "%s", err)
			return
		}
	}

	http.Redirect(w, req, "/ping", 302)
}

func (*app) UpdatePerson(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		util.HTTPError("ping", w, req, http.StatusBadRequest, "%s", err)
		return
	}

	slug := req.Form.Get("slug")
	isActive := req.Form.Get("is_active") == "on"

	if err := updatePerson(conn, slug, isActive); err != nil {
			util.HTTPError("ping", w, req, http.StatusInternalServerError, "%s", err)
			return
	}

	http.Redirect(w, req, fmt.Sprintf("/ping/person?slug=%s", slug), 302)
}

func (*app) PingPerson(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		util.HTTPError("ping", w, req, http.StatusBadRequest, "%s", err)
		return
	}

	slug := req.Form.Get("slug")
	notes := req.Form.Get("notes")

	person, _, err := showPerson(conn, slug)
	if err != nil {
		util.HTTPError("ping", w, req, http.StatusInternalServerError, "%s", err)
		return
	}

	if person.IsLongestUnpinged {
		if err := beeminder.Insert(beeminder.Datapoint{User: "ajm", Goal: "ping", Value: 1, Comment: slug}); err != nil {
			util.HTTPError("ping", w, req, http.StatusInternalServerError, "%s", err)
			return
		}
	}

	if err := addPing(conn, slug, notes); err != nil {
		util.HTTPError("ping", w, req, http.StatusInternalServerError, "%s", err)
		return
	}

	http.Redirect(w, req, fmt.Sprintf("/ping/person?slug=%s", slug), 302)
}
