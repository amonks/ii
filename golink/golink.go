package golink

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

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
	fmt.Println("init golink")
	ts, err := util.ReadTemplates(files, "templates")
	if err != nil {
		panic(err)
	}
	templates = ts
}

func Server() *dbserver.DBServer {
	fmt.Println("build golink server")
	s := dbserver.New("golink")
	a := &app{}
	s.HandleFunc("/go", a.Handler)
	s.HandleFunc("/go/", a.Handler)
	s.Start(a.Migrate)
	fmt.Println("started server")
	return s
}

type app struct{}

func (a *app) Migrate(conn *sqlite.Conn) error {
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

func (*app) Handler(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	log.Println("->", req.Method, req.URL)

	if req.Method == "GET" && req.URL.Path == "/go/" {
		log.Println("path: list")
		urls, err := List(req.Context(), conn)
		if err != nil {
			util.HTTPError("golink", w, req, http.StatusInternalServerError, "%s", err)
			return
		}

		v := struct{ Shortlinks []Shortening }{Shortlinks: urls}
		if err := templates["index.gohtml"].Execute(w, v); err != nil {
			util.HTTPError("golink", w, req, http.StatusInternalServerError, "%s", err)
			return
		}

		return
	}

	if req.Method == "POST" {
		log.Println("path: post")
		if err := req.ParseForm(); err != nil {
			util.HTTPError("golink", w, req, http.StatusBadRequest, "%s", err)
			return
		}

		key := req.Form.Get("key")
		url := req.Form.Get("url")

		if key == "" || url == "" {
			util.HTTPError("golink", w, req, http.StatusBadRequest, "%s")
			return
		}

		err := Set(req.Context(), conn, key, url)
		if err != nil {
			util.HTTPError("golink", w, req, http.StatusInternalServerError, "%s", err)
			return
		}

		http.Redirect(w, req, "/go", 302)
		return
	}

	if req.Method == "DELETE" {
		log.Println("path: del")
		key := strings.Trim(req.URL.Path, "/go/")

		if err := Delete(req.Context(), conn, key); err != nil {
			util.HTTPError("golink", w, req, http.StatusInternalServerError, "%s", err)
			return
		}

		w.WriteHeader(200)
		w.Write([]byte("ok"))
		return
	}

	if req.Method == "GET" {
		log.Println("path: get")
		key := strings.Trim(req.URL.Path, "/go/")

		url, err := Get(req.Context(), conn, key)
		if err != nil {
		log.Println("get err")
			util.HTTPError("golink", w, req, http.StatusInternalServerError, "%s", err)
			return
		}

		if url == "" {
		log.Println("no such")
			util.HTTPError("golink", w, req, http.StatusInternalServerError, "%s", err)
			return
		}

		http.Redirect(w, req, url, 301)
		return
	}

	log.Println("path: none")
	util.HTTPError("golink", w, req, http.StatusNotFound, "no such path")
}
