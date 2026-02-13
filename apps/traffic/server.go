package main

import (
	"embed"
	"encoding/json"
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

	//go:embed static/index.css
	indexCSS string
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
	s.HandleFunc("POST /log", s.handleLog)
	s.HandleFunc("GET /index.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(indexCSS))
	})
	return s
}

func (app *Server) serveTraffic(w http.ResponseWriter, req *http.Request) {
	tr := traffic.ParseTimeRange(req)

	var log []traffic.Request
	if err := app.model.
		Where("created_at >= ? AND created_at <= ?", tr.StartTime(), tr.EndTime()).
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
		Where("created_at >= ? AND created_at <= ?", tr.StartTime(), tr.EndTime()).
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
		Where("created_at >= ? AND created_at <= ?", tr.StartTime(), tr.EndTime()).
		Group("url").
		Limit(20).
		Order("request_count desc").
		Find(&topPages).
		Error; err != nil {
		serve.Errorf(w, req, 500, "failed to read top pages: %s", err)
		return
	}

	chartData, err := app.model.GetTrafficAggregates(tr)
	if err != nil {
		serve.Errorf(w, req, 500, "failed to read chart data: %s", err)
		return
	}
	chartJSON, err := json.Marshal(chartData)
	if err != nil {
		serve.Errorf(w, req, 500, "failed to marshal chart data: %s", err)
		return
	}

	type PageData struct {
		TimeRange traffic.TimeRange
		Log       []traffic.Request
		TopPages  []struct {
			URL          string
			RequestCount int
		}
		TopClients []struct {
			Client       string
			RequestCount int
		}
		ChartJSON template.JS
	}

	pageData := PageData{
		TimeRange:  tr,
		Log:        log,
		TopPages:   topPages,
		TopClients: topClients,
		ChartJSON:  template.JS("window.chartData = " + string(chartJSON) + ";"),
	}

	w.Header().Set("Content-type", "text/html; charset=utf-8")
	if err := templates["index.gohtml"].Execute(w, pageData); err != nil {
		serve.Errorf(w, req, 500, "failed to read template: %s", err)
		return
	}
}

func (app *Server) handleLog(w http.ResponseWriter, req *http.Request) {
	var entries []traffic.LogEntry
	if err := json.NewDecoder(req.Body).Decode(&entries); err != nil {
		serve.Errorf(w, req, 400, "bad request: %s", err)
		return
	}
	if err := app.model.LogEntries(entries); err != nil {
		serve.Errorf(w, req, 500, "failed to log entries: %s", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
