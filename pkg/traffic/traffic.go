package traffic

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"monks.co/pkg/color"
)

var RemoteAddrKey = &struct{}{}

type Request struct {
	gorm.Model

	CreatedAt *time.Time

	Host  string
	Path  string
	Query string

	RemoteAddr string
	UserAgent  string
	Referer    string

	StatusCode int
	Duration   time.Duration
}

func (r *Request) PrintDate() string {
	return r.CreatedAt.Format("2006-01-02 15:04:05")
}

func (r *Request) PrintDuration() string {
	return fmt.Sprintf("%dµs", r.Duration.Microseconds())
}

func (r *Request) PrintURL() string {
	if r.Query == "" {
		return r.Host + r.Path
	}
	return r.Host + r.Path + "?" + r.Query
}

func (r *Request) ColorRemoteAddr() string {
	return color.Hash(r.RemoteAddr)
}

func (r *Request) PrintUserAgent() string {
	return r.UserAgent
}

type TrafficLogger struct {
	db      *gorm.DB
	host    string
	handler http.Handler
}

func Open() (*gorm.DB, error) {
	dbPath := filepath.Join(os.Getenv("MONKS_DATA"), "traffic.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Request{}); err != nil {
		return nil, err
	}
	return db, nil
}

func New(host string, handler http.Handler) (*TrafficLogger, error) {
	db, err := Open()
	if err != nil {
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

		RemoteAddr: getRemoteAddr(req),
		UserAgent:  req.UserAgent(),
		Referer:    req.Header.Get("Referer"),

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

func getRemoteAddr(req *http.Request) string {
	ctx := req.Context()
	v := ctx.Value(RemoteAddrKey)
	switch v := v.(type) {
	case string:
		return v
	default:
		return req.RemoteAddr
	}
}
