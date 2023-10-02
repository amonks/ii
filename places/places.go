package places

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"crawshaw.io/sqlite"
	"monks.co/credentials"
	"monks.co/dbserver"
	"monks.co/util"
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
	model *model
}

func New() *server {
	m := NewModel()
	s := &server{
		DBServer: dbserver.New("places", m.migrate),
		model:    m,
	}

	s.HandleFunc("/places/index.js", s.JSServer("./places/ts/index.ts"))

	s.HandleFunc("/places/index.css", s.StaticServer("./places/static/"))
	s.HandleFunc("/places/dot.png", s.StaticServer("./places/static/"))

	s.HandleFunc("/places/", s.places)

	s.HandleFunc("/places/commands/import-saved-places", s.importSavedPlaces)
	s.HandleFunc("/places/commands/annotate-peoples-places", s.annotatePeoplesPlaces)

	return s
}

func (s *server) places(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("url")
	if id == "" {
		s.placesList(conn, w, req)
		return
	}

	place, err := s.model.getPlace(conn, id)
	if err != nil {
		s.InternalServerError(w, req, err)
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
		s.InternalServerError(w, req, err)
		return
	}
}

func (s *server) placesList(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	places, err := s.model.listPlaces(conn)
	if err != nil {
		s.InternalServerError(w, req, err)
		return
	}

	googleMapsImportURL := fmt.Sprintf(
		"https://maps.googleapis.com/maps/api/js?key=%s&callback=initMap&v=beta&libraries=marker",
		credentials.PlacesBrowserAPIKey,
	)

	placesJSON, err := json.Marshal(places)
	if err != nil {
		s.InternalServerError(w, req, err)
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
		s.InternalServerError(w, req, err)
		return
	}
}

func (s *server) importSavedPlaces(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := s.model.importSavedPlaces(conn); err != nil {
		s.InternalServerError(w, req, err)
		return
	}

	http.Redirect(w, req, "/places", 302)
}

func (s *server) annotatePeoplesPlaces(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := s.model.annotatePeoplesPlaces(conn); err != nil {
		s.InternalServerError(w, req, err)
		return
	}

	http.Redirect(w, req, "/places", 302)
}
