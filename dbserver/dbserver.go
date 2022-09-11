package dbserver

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

type DBServer struct {
	mux  *http.ServeMux
	name string
	db   *sqlitex.Pool
}

func New(name string) *DBServer {
	return &DBServer{
		mux:  http.NewServeMux(),
		name: name,
		db:   nil,
	}
}

func (s *DBServer) Start(migrate func(conn *sqlite.Conn) error) {
	db, err := sqlitex.Open(fmt.Sprintf("file:data/%s.db", s.name), 0, 10)
	if err != nil {
		log.Fatal(err)
	}

	s.db = db

	conn := db.Get(context.Background())
	if conn == nil {
		log.Fatal(err)
	}
	defer db.Put(conn)

	if err := migrate(conn); err != nil {
		log.Fatal(err)
	}
	fmt.Println("started db server")
}

func (s *DBServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Println("serving app", req.URL.Path)
	s.mux.ServeHTTP(w, req)
}

func (s *DBServer) HandleFunc(pattern string, f func(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request)) {
	fmt.Println("building handler func", pattern)
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		fmt.Println("executing handler func", pattern)
		if s.db == nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}

		conn := s.db.Get(req.Context())
		if conn == nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		defer s.db.Put(conn)

		f(conn, w, req)
	})
}
