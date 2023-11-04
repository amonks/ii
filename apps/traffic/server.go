package main

import (
	"embed"
	"html/template"
	"net/http"

	"monks.co/pkg/serve"
	"monks.co/pkg/traffic"
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

type Server struct {
	*http.ServeMux
	model *traffic.Model
}

func NewServer(m *traffic.Model) *Server {
	s := &Server{http.NewServeMux(), m}
	s.Handle("/index.css", serve.StaticServer("./static/"))
	s.Handle("/", s)
	return s
}

func (app *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var requests []traffic.Request
	if tx := app.model.Order("created_at desc").Find(&requests); tx.Error != nil {
		util.HTTPError("traffic", w, req, 500, "failed to read logs: %s", tx.Error)
		return
	}
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	if err := templates["index.gohtml"].Execute(w, requests); err != nil {
		util.HTTPError("traffic", w, req, 500, "failed to read template: %s", err)
		return
	}
}
