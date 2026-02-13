package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"sync"

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
	if req.URL.RawQuery == "" {
		http.Redirect(w, req, req.URL.Path+"?range=7d", http.StatusFound)
		return
	}

	tr := traffic.ParseTimeRange(req)

	var (
		wg       sync.WaitGroup
		logRows  []traffic.Request
		topPages []struct {
			URL          string
			RequestCount int
		}
		statusCodes []struct {
			StatusCode   int
			RequestCount int
		}
		durations []struct {
			DurationBucket int64
			RequestCount   int
		}
		chartData                                    map[string][]traffic.ChartPoint
		logErr, pagesErr, statusErr, durErr, chartErr error
	)

	wg.Add(5)
	go func() {
		defer wg.Done()
		logErr = app.model.
			Where("created_at >= ? AND created_at <= ?", tr.StartTime(), tr.EndTime()).
			Order("created_at desc").
			Limit(50).
			Find(&logRows).
			Error
	}()
	go func() {
		defer wg.Done()
		pagesErr = app.model.Raw(`
			SELECT host || path as url, SUM(count) as request_count
			FROM page_daily
			WHERE day >= ? AND day <= ?
			GROUP BY host, path
			ORDER BY request_count DESC
			LIMIT 20
		`, tr.StartTime(), tr.EndTime()).Scan(&topPages).Error
	}()
	go func() {
		defer wg.Done()
		statusErr = app.model.Raw(`
			SELECT status_code, SUM(count) as request_count
			FROM daily_stats
			WHERE day >= ? AND day <= ?
			GROUP BY status_code
			ORDER BY request_count DESC
		`, tr.StartTime(), tr.EndTime()).Scan(&statusCodes).Error
	}()
	go func() {
		defer wg.Done()
		durErr = app.model.Raw(`
			SELECT duration_bucket, SUM(count) as request_count
			FROM daily_stats
			WHERE day >= ? AND day <= ?
			GROUP BY duration_bucket
			ORDER BY duration_bucket ASC
		`, tr.StartTime(), tr.EndTime()).Scan(&durations).Error
	}()
	go func() {
		defer wg.Done()
		chartData, chartErr = app.model.GetChartData(tr)
	}()
	wg.Wait()

	if logErr != nil {
		serve.Errorf(w, req, 500, "failed to read logs: %s", logErr)
		return
	}
	if pagesErr != nil {
		serve.Errorf(w, req, 500, "failed to read top pages: %s", pagesErr)
		return
	}
	if statusErr != nil {
		serve.Errorf(w, req, 500, "failed to read status codes: %s", statusErr)
		return
	}
	if durErr != nil {
		serve.Errorf(w, req, 500, "failed to read durations: %s", durErr)
		return
	}
	if chartErr != nil {
		serve.Errorf(w, req, 500, "failed to read chart data: %s", chartErr)
		return
	}
	chartJSON, err := json.Marshal(chartData)
	if err != nil {
		serve.Errorf(w, req, 500, "failed to marshal chart data: %s", err)
		return
	}

	// Find max duration count for bar scaling
	maxDurCount := 0
	for _, d := range durations {
		if d.RequestCount > maxDurCount {
			maxDurCount = d.RequestCount
		}
	}

	type PageData struct {
		TimeRange   traffic.TimeRange
		Log         []traffic.Request
		TopPages    []struct {
			URL          string
			RequestCount int
		}
		StatusCodes []struct {
			StatusCode   int
			RequestCount int
		}
		Durations []struct {
			DurationBucket int64
			RequestCount   int
		}
		MaxDurCount int
		ChartJSON   template.JS
	}

	pageData := PageData{
		TimeRange:   tr,
		Log:         logRows,
		TopPages:    topPages,
		StatusCodes: statusCodes,
		Durations:   durations,
		MaxDurCount: maxDurCount,
		ChartJSON:   template.JS("window.chartData = " + string(chartJSON) + ";"),
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
