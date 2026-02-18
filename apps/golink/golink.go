package main

import (
	"embed"
	"html/template"
	"net/http"
	"strings"

	"monks.co/pkg/serve"
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
	*serve.Mux
	model *model
}

func NewServer(m *model) *server {
	s := &server{serve.NewMux(), m}
	s.HandleFunc("GET /{$}", s.handleList)
	s.HandleFunc("POST /", s.handlePost)
	s.HandleFunc("DELETE /", s.handleDelete)
	s.HandleFunc("GET /", s.handleGet)
	return s
}

func (s *server) handleList(w http.ResponseWriter, req *http.Request) {
	urls, err := s.model.List()
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	v := struct {
		Shortlinks []Shortening
		BasePath   string
	}{Shortlinks: urls, BasePath: serve.BasePath(req)}
	if err := templates["index.gohtml"].Execute(w, v); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}
}

func (s *server) handlePost(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		serve.Error(w, req, http.StatusBadRequest, err)
		return
	}

	key := req.Form.Get("key")
	url := req.Form.Get("url")

	if key == "" || url == "" {
		serve.Errorf(w, req, http.StatusBadRequest, "key or url is required")
		return
	}

	err := s.model.Set(key, url)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	http.Redirect(w, req, "/", http.StatusFound)
}

func (s *server) handleDelete(w http.ResponseWriter, req *http.Request) {
	key := strings.TrimPrefix(req.URL.Path, "/")

	if err := s.model.Delete(key); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	w.WriteHeader(200)
}

func (s *server) handleGet(w http.ResponseWriter, req *http.Request) {
	key := strings.TrimPrefix(req.URL.Path, "/")

	url, err := s.model.Get(key)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	if url == "" {
		serve.Error(w, req, http.StatusNotFound, err)
		return
	}

	http.Redirect(w, req, url, http.StatusMovedPermanently)
}
