package main

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"monks.co/pkg/beeminder"
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

	s.HandleFunc("GET /{$}", s.ListPeople)
	s.HandleFunc("GET /person/{$}", s.ShowPerson)
	s.HandleFunc("POST /commands/bump/{$}", s.Bump)
	s.HandleFunc("POST /commands/ping-person/{$}", s.PingPerson)
	s.HandleFunc("POST /commands/add-person/{$}", s.AddPerson)
	s.HandleFunc("POST /commands/update-person/{$}", s.UpdatePerson)

	return s
}

func (s *server) ListPeople(w http.ResponseWriter, req *http.Request) {
	people, err := s.model.listPeople()
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	if err := templates["list.gohtml"].Execute(w, people); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}
}

func (s *server) ShowPerson(w http.ResponseWriter, req *http.Request) {
	slug := req.URL.Query().Get("slug")

	person, pings, err := s.model.showPerson(slug)
	if err != nil {
		serve.InternalServerError(w, req, err)
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
		serve.InternalServerError(w, req, err)
		return
	}
}

func (s *server) AddPerson(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		serve.Error(w, req, http.StatusBadRequest, err)
		return
	}

	slug := req.Form.Get("slug")
	notes := req.Form.Get("notes")

	if err := s.model.addPerson(slug); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	if notes != "" {
		if err := s.model.addPing(slug, notes); err != nil {
			serve.InternalServerError(w, req, err)
			return
		}
	}

	http.Redirect(w, req, "/ping", http.StatusFound)
}

func (s *server) UpdatePerson(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		serve.Error(w, req, http.StatusBadRequest, err)
		return
	}

	slug := req.Form.Get("slug")
	isActive := req.Form.Get("is_active") == "on"

	if err := s.model.updatePerson(slug, isActive); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	http.Redirect(w, req, fmt.Sprintf("/ping/person?slug=%s", slug), http.StatusFound)
}

func (s *server) Bump(w http.ResponseWriter, req *http.Request) {
	if err := beeminder.Insert(beeminder.Datapoint{User: "ajm", Goal: "ping", Value: 1, Comment: "bump"}); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	http.Redirect(w, req, "/ping", http.StatusFound)
}

func (s *server) PingPerson(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		serve.Error(w, req, http.StatusBadRequest, err)
		return
	}

	slug := req.Form.Get("slug")
	notes := req.Form.Get("notes")

	person, _, err := s.model.showPerson(slug)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	if person.IsLongestUnpinged {
		if err := beeminder.Insert(beeminder.Datapoint{User: "ajm", Goal: "ping", Value: 1, Comment: slug}); err != nil {
			serve.InternalServerError(w, req, err)
			return
		}
	}

	if err := s.model.addPing(slug, notes); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	http.Redirect(w, req, fmt.Sprintf("/ping/person/?slug=%s", slug), http.StatusFound)
}
