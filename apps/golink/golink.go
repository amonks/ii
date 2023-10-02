package golink

import (
	"embed"
	"html/template"
	"net/http"
	"strings"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
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
		dbserver.New("golink", migrate),
	}

	s.HandleFunc("/go", s.Handler)
	s.HandleFunc("/go/", s.Handler)

	return s
}

func migrate(conn *sqlite.Conn) error {
	if err := sqlitex.ExecScript(conn, `
		create table if not exists urls (
			key text primary key not null,
			url text,
			created_at datetime
		);`,
	); err != nil {
		return err
	}
	return nil
}

func (s *server) Handler(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	s.Logf("-> %s %s", req.Method, req.URL)

	if req.Method == "GET" && req.URL.Path == "/go/" {
		s.Logf("path: list")
		urls, err := List(req.Context(), conn)
		if err != nil {
			s.InternalServerError(w, req, err)
			return
		}

		v := struct{ Shortlinks []Shortening }{Shortlinks: urls}
		if err := templates["index.gohtml"].Execute(w, v); err != nil {
			s.InternalServerError(w, req, err)
			return
		}

		return
	}

	if req.Method == "POST" {
		s.Logf("path: post")
		if err := req.ParseForm(); err != nil {
			s.Error(w, req, http.StatusBadRequest, err)
			return
		}

		key := req.Form.Get("key")
		url := req.Form.Get("url")

		if key == "" || url == "" {
			s.Errorf(w, req, http.StatusBadRequest, "key or url is required")
			return
		}

		err := Set(req.Context(), conn, key, url)
		if err != nil {
			s.InternalServerError(w, req, err)
			return
		}

		http.Redirect(w, req, "/go", 302)
		return
	}

	if req.Method == "DELETE" {
		s.Logf("path: del")
		key := strings.Trim(req.URL.Path, "/go/")

		if err := Delete(req.Context(), conn, key); err != nil {
			s.InternalServerError(w, req, err)
			return
		}

		w.WriteHeader(200)
		return
	}

	if req.Method == "GET" {
		s.Logf("path: get '%s'", req.URL.Path)
		key := strings.Trim(req.URL.Path, "/go/")

		url, err := Get(req.Context(), conn, key)
		if err != nil {
			s.Logf("no such link: '%s'", key)
			s.InternalServerError(w, req, err)
			return
		}

		if url == "" {
			s.Logf("no match for key: '%s'", key)
			s.Error(w, req, http.StatusNotFound, err)
			return
		}

		http.Redirect(w, req, url, 301)
		return
	}

	s.Errorf(w, req, http.StatusNotFound, "no such path")
}
