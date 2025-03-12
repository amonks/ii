package main

import (
	"embed"
	"html/template"
	"net/http"

	"monks.co/pkg/serve"
	"monks.co/pkg/traffic"
	"monks.co/pkg/util"
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

type Server struct {
	*serve.Mux
	model *traffic.Model
}

func NewServer(m *traffic.Model) *Server {
	s := &Server{serve.NewMux(), m}
	s.HandleFunc("GET /{$}", s.serveTraffic)
	s.Handle("GET /index.css", serve.StaticServer("./static/"))
	return s
}

func (app *Server) serveTraffic(w http.ResponseWriter, req *http.Request) {
	var log []traffic.Request
	if err := app.model.
		Order("created_at desc").
		Limit(50).
		Find(&log).
		Error; err != nil {
		serve.Errorf(w, req, 500, "failed to read logs: %s", err)
		return
	}

	var topClients []struct {
		Client       string
		RequestCount int
	}
	if err := app.model.Table("requests").
		Select("case when instr(remote_addr, ']') then substr(remote_addr, 0, instr(remote_addr, ']')+1) when instr(remote_addr, ':') then substr(remote_addr, 0, instr(remote_addr, ':')) else remote_addr end as client, count(*) as request_count").
		Where("created_at > datetime('now', '-7 day')").
		Group("client").
		Limit(20).
		Order("request_count desc").
		Find(&topClients).
		Error; err != nil {
		serve.Errorf(w, req, 500, "failed to read top clients: %s", err)
		return
	}

	var topPages []struct {
		URL          string
		RequestCount int
	}
	if err := app.model.Table("requests").
		Select("host || path as url, count(*) as request_count").
		Where("created_at > datetime('now', '-7 day')").
		Group("url").
		Limit(20).
		Order("request_count desc").
		Find(&topPages).
		Error; err != nil {
		serve.Errorf(w, req, 500, "failed to read top pages: %s", err)
		return
	}

	type PageData struct {
		Log      []traffic.Request
		TopPages []struct {
			URL          string
			RequestCount int
		}
		TopClients []struct {
			Client       string
			RequestCount int
		}
	}

	pageData := PageData{
		Log:        log,
		TopPages:   topPages,
		TopClients: topClients,
	}

	w.Header().Set("Content-type", "text/html; charset=utf-8")
	if err := templates["index.gohtml"].Execute(w, pageData); err != nil {
		serve.Errorf(w, req, 500, "failed to read template: %s", err)
		return
	}
}
