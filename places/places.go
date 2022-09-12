package places

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

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
	fmt.Println("init places")
	ts, err := util.ReadTemplates(files, "templates")
	if err != nil {
		panic(err)
	}
	templates = ts
}

func Server() *dbserver.DBServer {
	fmt.Println("build places server")
	s := dbserver.New("places")
	a := &app{}
	s.HandleFunc("/places/", a.Places)
	s.HandleFunc("/places/commands/import-saved-places", a.ImportSavedPlaces)
	s.HandleFunc("/places/commands/annotate-peoples-places", a.AnnotatePeoplesPlaces)
	s.Start(a.Migrate)
	fmt.Println("started server")
	return s
}

type app struct{}

func (a *app) Migrate(conn *sqlite.Conn) error {
	if err := sqlitex.ExecScript(conn, `
		create table if not exists places (
			google_maps_url text primary key not null,
			google_maps_place_id text,
			google_maps_business_status text,
			is_public integer,
			notes text,
			rating integer,
			created_at text,
			updated_at text,
			lat text,
			lng text,
			business_name text,
			country_code text,
			address text,
			title text
		);`); err != nil {
		return err
	}
	return nil
}

func (*app) Places(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	places, err := listPlaces(conn)
	if err != nil {
		util.HTTPError("places", w, req, http.StatusInternalServerError, "%s", err)
	}

	if err := templates["list.gohtml"].Execute(w, places); err != nil {
		util.HTTPError("places", w, req, http.StatusInternalServerError, "%s", err)
		return
	}
}

func (*app) ImportSavedPlaces(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := importSavedPlaces(conn); err != nil {
		util.HTTPError("places", w, req, http.StatusInternalServerError, "%s", err)
		return
	}

	http.Redirect(w, req, "/places", 302)
}

func (*app) AnnotatePeoplesPlaces(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if err := annotatePeoplesPlaces(conn); err != nil {
		util.HTTPError("places", w, req, http.StatusInternalServerError, "%s", err)
		return
	}

	http.Redirect(w, req, "/places", 302)
}
