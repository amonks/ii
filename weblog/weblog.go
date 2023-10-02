package weblog

import (
	"errors"
	"net/http"
	"strings"

	"crawshaw.io/sqlite"
	"monks.co/dbserver"
)

type server struct {
	*dbserver.DBServer
	model *model
}

func New() (*server) {
	m := NewModel()
	s := &server{
		DBServer: dbserver.New("weblog", m.migrate),
		model:    m,
	}

	s.HandleFunc("/.well-known/webfinger", s.serveWebfinger)
	s.HandleFunc("/", s.serveRoot)

	return s
}

func (s *server) serveWebfinger(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	resource := req.URL.Query().Get("resource")
	if resource != "a@monks.co" {
		s.Error(w, req, 404, errors.New("no such resource"))
		return
	}

	w.Header().Add("Content-Type", "application/jrd+json")
	w.Write(getWebfinger())
}

func (s *server) serveRoot(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/" {
		accept := req.Header.Get("Accept")
		if strings.Contains(accept, "application/activity+json") || strings.Contains(accept, `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`) {
			bs, err := getActor()
			if err != nil {
				s.Error(w, req, http.StatusInternalServerError, err)
				return
			}
			w.Header().Add("Content-Type", "application/activity+json")
			w.Write(bs)
			return
		}
	}

	http.FileServer(http.Dir("./static")).ServeHTTP(w, req)
}

// func (s *server) serveHomepage
