package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"monks.co/pkg/color"
	"monks.co/pkg/logs"
	"monks.co/pkg/serve"
)

var (
	//go:embed templates/*
	files     embed.FS
	templates map[string]*template.Template

	//go:embed static/index.css
	indexCSS string
)

func init() {
	funcMap := template.FuncMap{
		"divide": func(a, b int64) int64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"percent": func(a, b int) string {
			if b == 0 {
				return "0"
			}
			return fmt.Sprintf("%.1f", float64(a)/float64(b)*100)
		},
		"colorHash": color.Hash,
		"formatDuration": func(ms *float64) string {
			if ms == nil {
				return ""
			}
			if *ms < 1 {
				return fmt.Sprintf("%.0fus", *ms*1000)
			}
			if *ms < 1000 {
				return fmt.Sprintf("%.0fms", *ms)
			}
			return fmt.Sprintf("%.2fs", *ms/1000)
		},
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"deref": func(s *string) string {
			if s == nil {
				return ""
			}
			return *s
		},
		"derefInt": func(i *int) int {
			if i == nil {
				return 0
			}
			return *i
		},
		"prettyJSON": func(raw logs.JSONText) string {
			var buf bytes.Buffer
			if err := json.Indent(&buf, []byte(raw), "", "  "); err != nil {
				return string(raw)
			}
			return buf.String()
		},
	}
	t, err := template.New("index.gohtml").Funcs(funcMap).ParseFS(files, "templates/index.gohtml")
	if err != nil {
		panic(err)
	}
	traceT, err := template.New("trace.gohtml").Funcs(funcMap).ParseFS(files, "templates/trace.gohtml")
	if err != nil {
		panic(err)
	}
	templates = map[string]*template.Template{
		"index.gohtml": t,
		"trace.gohtml": traceT,
	}
}

type Server struct {
	*serve.Mux
	model *logs.Model
}

func NewServer(m *logs.Model) *Server {
	s := &Server{serve.NewMux(), m}
	s.HandleFunc("GET /{$}", s.serveDashboard)
	s.HandleFunc("GET /query", s.handleQuery)
	s.HandleFunc("GET /values", s.handleValues)
	s.HandleFunc("POST /ingest", s.handleIngest)
	s.HandleFunc("GET /trace/{id}", s.serveTrace)
	s.HandleFunc("GET /index.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(indexCSS))
	})
	return s
}

func (app *Server) serveDashboard(w http.ResponseWriter, req *http.Request) {
	if req.URL.RawQuery == "" {
		w.Header().Set("Location", "?range=7d")
		w.WriteHeader(http.StatusFound)
		return
	}

	tr := logs.ParseTimeRange(req)

	var query logs.Query
	if qs := req.URL.Query().Get("q"); qs != "" {
		query = logs.ParseQuery(qs)
	} else {
		query = logs.Query{GroupBy: "host"}
	}

	var (
		wg       sync.WaitGroup
		events   []logs.Event
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
		eventsErr, pagesErr, statusErr, durErr error
	)

	var chartData map[string][]logs.ChartPoint
	var chartErr error

	wg.Add(5)
	go func() {
		defer wg.Done()
		events, eventsErr = app.model.GetRecentEvents(tr, 50)
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
			SELECT status as status_code, SUM(count) as request_count
			FROM daily_stats
			WHERE day >= ? AND day <= ?
			GROUP BY status
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
		chartData, chartErr = app.model.QueryChartData(tr, query)
	}()
	wg.Wait()

	if eventsErr != nil {
		serve.Errorf(w, req, 500, "failed to read events: %s", eventsErr)
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
		serve.Errorf(w, req, 500, "failed to run query: %s", chartErr)
		return
	}

	chartJSON, err := json.Marshal(chartData)
	if err != nil {
		serve.Errorf(w, req, 500, "failed to marshal chart data: %s", err)
		return
	}
	queryJSON, err := json.Marshal(query)
	if err != nil {
		serve.Errorf(w, req, 500, "failed to marshal query: %s", err)
		return
	}

	totalStatusCount := 0
	for _, s := range statusCodes {
		totalStatusCount += s.RequestCount
	}
	totalDurCount := 0
	for _, d := range durations {
		totalDurCount += d.RequestCount
	}

	type PageData struct {
		TimeRange   logs.TimeRange
		Events      []logs.Event
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
		TotalStatusCount int
		TotalDurCount    int
		ChartJSON        template.JS
		QueryJSON        template.JS
	}

	pageData := PageData{
		TimeRange:        tr,
		Events:           events,
		TopPages:         topPages,
		StatusCodes:      statusCodes,
		Durations:        durations,
		TotalStatusCount: totalStatusCount,
		TotalDurCount:    totalDurCount,
		ChartJSON:        template.JS(string(chartJSON)),
		QueryJSON:        template.JS(string(queryJSON)),
	}

	var buf bytes.Buffer
	if err := templates["index.gohtml"].Execute(&buf, pageData); err != nil {
		serve.Errorf(w, req, 500, "failed to execute template: %s", err)
		return
	}
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func (app *Server) handleQuery(w http.ResponseWriter, req *http.Request) {
	tr := logs.ParseTimeRange(req)
	qs := req.URL.Query().Get("q")
	if qs == "" {
		serve.Errorf(w, req, 400, "missing q parameter")
		return
	}
	q := logs.ParseQuery(qs)
	data, err := app.model.QueryChartData(tr, q)
	if err != nil {
		serve.Errorf(w, req, 500, "query error: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (app *Server) handleValues(w http.ResponseWriter, req *http.Request) {
	tr := logs.ParseTimeRange(req)
	dim := req.URL.Query().Get("dim")
	if dim == "" {
		serve.Errorf(w, req, 400, "missing dim parameter")
		return
	}
	vals, err := app.model.GetDimensionValues(tr, dim)
	if err != nil {
		serve.Errorf(w, req, 500, "values error: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vals)
}

func (app *Server) handleIngest(w http.ResponseWriter, req *http.Request) {
	var events []json.RawMessage
	if err := json.NewDecoder(req.Body).Decode(&events); err != nil {
		serve.Errorf(w, req, 400, "bad request: %s", err)
		return
	}
	if err := app.model.Ingest(events); err != nil {
		serve.Errorf(w, req, 500, "failed to ingest events: %s", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (app *Server) serveTrace(w http.ResponseWriter, req *http.Request) {
	requestID := req.PathValue("id")
	if requestID == "" {
		serve.Errorf(w, req, 400, "missing request ID")
		return
	}

	events, err := app.model.GetTrace(requestID)
	if err != nil {
		serve.Errorf(w, req, 500, "failed to get trace: %s", err)
		return
	}

	type TraceData struct {
		RequestID string
		Events    []logs.Event
	}

	data := TraceData{
		RequestID: requestID,
		Events:    events,
	}

	var buf bytes.Buffer
	if err := templates["trace.gohtml"].Execute(&buf, data); err != nil {
		serve.Errorf(w, req, 500, "failed to execute template: %s", err)
		return
	}
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}
