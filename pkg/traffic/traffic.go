package traffic

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Request struct {
	gorm.Model

	Host  string
	Path  string
	Query string

	RemoteAddr string
	UserAgent  string

	StatusCode int
	Duration   time.Duration
}

type TrafficLogger struct {
	db      *gorm.DB
	host    string
	handler http.Handler
}

func New(host string, handler http.Handler) (*TrafficLogger, error) {
	dbPath := filepath.Join(os.Getenv("MONKS_DATA"), "traffic.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Request{}); err != nil {
		return nil, err
	}
	return &TrafficLogger{db, host, handler}, nil
}

func (tl *TrafficLogger) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ww := &StatusRecorder{w, 0}

	start := time.Now()
	tl.handler.ServeHTTP(ww, req)
	dur := time.Since(start)

	if tx := tl.db.Create(&Request{
		Host:  req.Host,
		Path:  req.URL.Path,
		Query: req.URL.RawQuery,

		RemoteAddr: req.RemoteAddr,
		UserAgent:  req.UserAgent(),

		StatusCode: ww.status,
		Duration:   dur,
	}); tx.Error != nil {
		fmt.Println("error", tx.Error)
	}
}

type StatusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *StatusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
