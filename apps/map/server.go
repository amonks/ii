package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"monks.co/apps/map/model"
	"monks.co/pkg/serve"
	"monks.co/pkg/util"
)

var (
	//go:embed templates/*
	files     embed.FS
	templates map[string]*template.Template

	//go:embed static/index.js
	indexJS string
	//go:embed static/index.css
	indexCSS string
	//go:embed static/dot.png
	dotPNG []byte
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
	model *model.Model
}

func NewServer(m *model.Model) *server {
	s := &server{serve.NewMux(), m}

	s.HandleFunc("GET /index.js", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeContent(w, req, "index.js", time.Time{}, strings.NewReader(indexJS))
	})
	s.HandleFunc("GET /index.css", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		http.ServeContent(w, req, "index.css", time.Time{}, strings.NewReader(indexCSS))
	})
	s.HandleFunc("GET /dot.png", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		http.ServeContent(w, req, "dot.png", time.Time{}, bytes.NewReader(dotPNG))
	})

	s.HandleFunc("GET /{$}", s.places)

	return s
}

func (s *server) places(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("url")
	if id == "" {
		s.placesList(w, req)
		return
	}

	place, err := s.model.GetPlace(id)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	placeTemplateData := struct {
		Place *model.Place
	}{
		Place: place,
	}

	isAdmin := false
	template := "details.gohtml"
	if isAdmin {
		template = "form.gohtml"
	}

	if err := templates[template].Execute(w, placeTemplateData); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}
}

func (s *server) placesList(w http.ResponseWriter, req *http.Request) {
	places, err := s.model.ListPlaces()
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	log.Printf("%d places", len(places))

	googleMapsImportURL := fmt.Sprintf(
		"https://maps.googleapis.com/maps/api/js?key=%s&callback=initMap&v=beta&libraries=marker",
		placesBrowserAPIKey,
	)

	placesJSON, err := json.Marshal(places)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	placesTemplateData := struct {
		GoogleMapsImportURL string
		Places              []model.Place
		PlacesJSON          template.JS
	}{
		GoogleMapsImportURL: googleMapsImportURL,
		Places:              places,
		PlacesJSON:          template.JS(placesJSON),
	}
	if err := templates["list.gohtml"].Execute(w, placesTemplateData); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}
}
