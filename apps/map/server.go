package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"monks.co/apps/map/model"
	"monks.co/credentials"
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
	model *model.Model
}

func NewServer(m *model.Model) *server {
	s := &server{serve.NewMux(), m}

	s.Handle("GET /index.js", serve.JSServer("./ts/index.ts"))

	s.Handle("GET /index.css", serve.StaticServer("./static/"))
	s.Handle("GET /dot.png", serve.StaticServer("./static/"))

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
		credentials.PlacesBrowserAPIKey,
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
