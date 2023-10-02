package dbserver

import (
	"context"
	"fmt"
	"net/http"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"monks.co/pkg/config"
	"monks.co/pkg/logger"
)

type DBServer struct {
	mux        *http.ServeMux
	name       string
	logger     logger.Logger
	db         *sqlitex.Pool
	migrate    func(conn *sqlite.Conn) error
	middleware []HTTPMiddleware
}

type HTTPMiddleware func(http.Handler) http.Handler

func New(name string, migrate func(conn *sqlite.Conn) error) *DBServer {
	return &DBServer{
		mux:        http.NewServeMux(),
		logger:     logger.New(name),
		name:       name,
		db:         nil,
		migrate:    migrate,
		middleware: nil,
	}
}

func (s *DBServer) Logf(msg string, args ...interface{}) {
	s.logger.Logf(msg, args...)
}

func (s *DBServer) AddMiddleware(mw func(http.Handler) http.Handler) {
	s.middleware = append(s.middleware, mw)
}

func (s *DBServer) Start(ctx context.Context) error {
	db, err := sqlitex.Open(fmt.Sprintf("file:%s/%s.db", config.Current.StoragePath, s.name), 0, 10)
	if err != nil {
		return err
	}

	s.db = db

	conn := db.Get(ctx)
	if conn == nil {
		return fmt.Errorf("failed to get connection")
	}
	defer db.Put(conn)

	if err := sqlitex.Exec(conn, `PRAGMA journal_mode=wal;`, nil); err != nil {
		return err
	}

	if err := sqlitex.Exec(conn, `PRAGMA synchronous=normal;`, nil); err != nil {
		return err
	}

	if err := s.migrate(conn); err != nil {
		return err
	}

	s.Logf("initialized")
	return nil
}

func (s *DBServer) Stop() error {
	return s.db.Close()
}

func (s *DBServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.Logf("serving: %s", req.URL.Path)

	var h http.Handler
	h = s.mux
	for _, mw := range s.middleware {
		h = mw(h)
	}

	h.ServeHTTP(w, req)
}

func (s *DBServer) Handle(pattern string, h http.Handler) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		s.Logf("url: %s", req.URL.String())
		if s.db == nil {
			s.InternalServerErrorf(w, req, "Internal error")
			return
		}

		h.ServeHTTP(w, req)
	})
}

func (s *DBServer) HandleFunc(pattern string, f func(conn *sqlite.Conn, w http.ResponseWriter, req *http.Request)) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		s.Logf("url: %s", req.URL.String())
		if s.db == nil {
			s.InternalServerErrorf(w, req, "Internal error")
			return
		}

		conn := s.db.Get(req.Context())
		if conn == nil {
			s.InternalServerErrorf(w, req, "Internal error")
			return
		}
		defer s.db.Put(conn)

		f(conn, w, req)
	})
}
