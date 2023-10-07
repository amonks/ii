package dbserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
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

func (s *DBServer) Listen(ctx context.Context, port int) error {
	srv := http.Server{Handler: s, Addr: fmt.Sprintf("0.0.0.0:%d", port)}

	done := make(chan error)
	go func() {
		fmt.Printf("listening on %d\n", port)
		if err := srv.ListenAndServe(); err != nil {
			done <- err
		}
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	}
}

func (s *DBServer) Logf(msg string, args ...interface{}) {
	s.logger.Logf(msg, args...)
}

func (s *DBServer) AddMiddleware(mw func(http.Handler) http.Handler) {
	s.middleware = append(s.middleware, mw)
}

func (s *DBServer) Run(ctx context.Context, port int) error {
	if err := s.Start(ctx); err != nil {
		return err
	}
	if err := s.Listen(ctx, port); err != nil {
		return err
	}
	return nil
}

func (s *DBServer) Start(ctx context.Context) error {
	// TODO: storagepath
	dbPath := filepath.Join(os.Getenv("MONKS_DATA"), s.name+".db")
	db, err := sqlitex.Open(dbPath, 0, 10)
	if err != nil {
		return err
	} else if err := ctx.Err(); err != nil {
		return err
	}

	s.db = db

	conn := db.Get(ctx)
	if conn == nil {
		return fmt.Errorf("failed to get connection")
	} else if err := ctx.Err(); err != nil {
		return err
	}
	defer db.Put(conn)

	if err := sqlitex.Exec(conn, `PRAGMA journal_mode=wal;`, nil); err != nil {
		return err
	} else if err := ctx.Err(); err != nil {
		return err
	}

	if err := sqlitex.Exec(conn, `PRAGMA synchronous=normal;`, nil); err != nil {
		return err
	} else if err := ctx.Err(); err != nil {
		return err
	}

	if err := s.migrate(conn); err != nil {
		return err
	} else if err := ctx.Err(); err != nil {
		return err
	}

	s.Logf("initialized")
	return nil
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
