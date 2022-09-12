package places

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"co.monks.monks.co/dbserver"
	"co.monks.monks.co/util"
	"crawshaw.io/sqlite"
)

var (
	//go:embed templates/*
	files     embed.FS
	templates map[string]*template.Template
)

func init() {
	fmt.Println("init places")
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

func Server() *server {
	fmt.Println("build places server")
	s := &server{
		DBServer: dbserver.New("places"),
	}
	s.HandleFunc("/places/", s.places)
	s.HandleFunc("/places/commands/import-saved-places", s.importSavedPlaces)
	s.HandleFunc("/places/commands/annotate-peoples-places", s.annotatePeoplesPlaces)
	s.Init(s.model.migrate)
	fmt.Println("started places server")
	return s
}

func (s *server) places(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	places, err := s.model.listPlaces(conn)
	if err != nil {
		util.HTTPError("places", w, req, http.StatusInternalServerError, "%s", err)
		return
	}

	if err := templates["list.gohtml"].Execute(w, places); err != nil {
		util.HTTPError("places", w, req, http.StatusInternalServerError, "%s", err)
		return
	}
}

func (s *server) importSavedPlaces(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := s.model.importSavedPlaces(conn); err != nil {
		util.HTTPError("places", w, req, http.StatusInternalServerError, "%s", err)
		return
	}

	http.Redirect(w, req, "/places", 302)
}

func (s *server) annotatePeoplesPlaces(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := s.model.annotatePeoplesPlaces(conn); err != nil {
		util.HTTPError("places", w, req, http.StatusInternalServerError, "%s", err)
		return
	}

	http.Redirect(w, req, "/places", 302)
}
