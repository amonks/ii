package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"

	"monks.co/credentials"
	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
	"monks.co/pkg/util"
)

var port = flag.Int("port", 3000, "port")

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	flag.Parse()
	s, err := newServer()
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}
	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	if err := http.ListenAndServe(addr, gzip.Handler(s)); err != nil {
		return fmt.Errorf("serving http: %w", err)
	}
	return nil
}

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
	*http.ServeMux
	model *model
}

func newServer() (*server, error) {
	m, err := NewModel()
	if err != nil {
		return nil, fmt.Errorf("constructing model: %w", err)
	}

	s := &server{http.NewServeMux(), m}

	s.Handle("/index.js", serve.JSServer("./ts/index.ts"))

	s.Handle("/index.css", serve.StaticServer("./static/"))
	s.Handle("/dot.png", serve.StaticServer("./static/"))

	s.HandleFunc("/", s.places)

	s.HandleFunc("/commands/import-saved-places", s.importSavedPlaces)
	s.HandleFunc("/commands/annotate-peoples-places", s.annotatePeoplesPlaces)

	return s, nil
}

func (s *server) places(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("url")
	if id == "" {
		s.placesList(w, req)
		return
	}

	place, err := s.model.getPlace(id)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	placeTemplateData := struct {
		Place *Place
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
	places, err := s.model.listPlaces()
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

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
		Places              []Place
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

func (s *server) importSavedPlaces(w http.ResponseWriter, req *http.Request) {
	if err := s.model.importSavedPlaces(); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	http.Redirect(w, req, "/places", 302)
}

func (s *server) annotatePeoplesPlaces(w http.ResponseWriter, req *http.Request) {
	if err := s.model.annotatePeoplesPlaces(); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	http.Redirect(w, req, "/places", 302)
}
