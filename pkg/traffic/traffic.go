package traffic

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"gorm.io/gorm"
	"monks.co/pkg/color"
	"monks.co/pkg/database"
	"monks.co/pkg/middleware"
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

func (r *Request) PrintRemoteAddr() string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (r *Request) ColorRemoteAddr() string {
	return color.Hash(r.PrintRemoteAddr())
}

func (r *Request) PrintUserAgent() string {
	return r.UserAgent
}

var _ middleware.Middleware = &TrafficLogger{}

type TrafficLogger struct {
	model *Model
	host  string
}

type Model struct {
	*database.DB
}

func Open() (*Model, error) {
	db, err := database.OpenFromDataFolder("traffic")
	if err != nil {
		return nil, err
	}
	return &Model{db}, nil
}

func New(host string) (*TrafficLogger, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}
	return &TrafficLogger{db, host}, nil
}

func (tl *TrafficLogger) Close() error {
	return tl.model.Close()
}

func (tl *TrafficLogger) ModifyHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ww := &StatusRecorder{w, 0}

		start := time.Now()
		handler.ServeHTTP(ww, req)
		dur := time.Since(start)

		if tx := tl.model.Create(&Request{
			Host:  req.Host,
			Path:  req.URL.Path,
			Query: req.URL.RawQuery,

			RemoteAddr: getRemoteAddr(req),
			UserAgent:  req.UserAgent(),
			Referer:    req.Header.Get("Referer"),

			StatusCode: ww.status,
			Duration:   dur,
		}); tx.Error != nil {
			log.Println("error", tx.Error)
		}
	})
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
