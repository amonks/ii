package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"

	"monks.co/pkg/serve"
	"monks.co/pkg/traffic"
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
	}
	t, err := template.New("index.gohtml").Funcs(funcMap).ParseFS(files, "templates/index.gohtml")
	if err != nil {
		panic(err)
	}
	templates = map[string]*template.Template{"index.gohtml": t}
}

type Server struct {
	*serve.Mux
	model *traffic.Model
}

func NewServer(m *traffic.Model) *Server {
	s := &Server{serve.NewMux(), m}
	s.HandleFunc("GET /{$}", s.serveTraffic)
	s.HandleFunc("GET /query", s.handleQuery)
	s.HandleFunc("GET /values", s.handleValues)
	s.HandleFunc("POST /log", s.handleLog)
	s.HandleFunc("GET /index.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(indexCSS))
	})
	return s
}

func (app *Server) serveTraffic(w http.ResponseWriter, req *http.Request) {
	if req.URL.RawQuery == "" {
		w.Header().Set("Location", "?range=7d")
		w.WriteHeader(http.StatusFound)
		return
	}

	tr := traffic.ParseTimeRange(req)

	// Parse query param.
	var query traffic.Query
	if qs := req.URL.Query().Get("q"); qs != "" {
		query = traffic.ParseQuery(qs)
	} else {
		query = traffic.Query{GroupBy: "host"}
	}

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
		logErr, pagesErr, statusErr, durErr error
	)

	var chartData map[string][]traffic.ChartPoint
	var chartErr error

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
		chartData, chartErr = app.model.QueryChartData(tr, query)
	}()
	wg.Wait()

	if logErr != nil {
		log.Printf("serveTraffic: logErr: %.200s", fmt.Sprint(logErr))
		serve.Errorf(w, req, 500, "failed to read logs: %s", logErr)
		return
	}
	if pagesErr != nil {
		log.Printf("serveTraffic: pagesErr: %.200s", fmt.Sprint(pagesErr))
		serve.Errorf(w, req, 500, "failed to read top pages: %s", pagesErr)
		return
	}
	if statusErr != nil {
		log.Printf("serveTraffic: statusErr: %.200s", fmt.Sprint(statusErr))
		serve.Errorf(w, req, 500, "failed to read status codes: %s", statusErr)
		return
	}
	if durErr != nil {
		log.Printf("serveTraffic: durErr: %.200s", fmt.Sprint(durErr))
		serve.Errorf(w, req, 500, "failed to read durations: %s", durErr)
		return
	}

	if chartErr != nil {
		log.Printf("serveTraffic: chartErr: %.200s", fmt.Sprint(chartErr))
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
		TotalStatusCount int
		TotalDurCount    int
		ChartJSON        template.JS
		QueryJSON        template.JS
	}

	pageData := PageData{
		TimeRange:        tr,
		Log:              logRows,
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
		log.Printf("serveTraffic: template error: %.200s", fmt.Sprint(err))
		serve.Errorf(w, req, 500, "failed to execute template: %s", err)
		return
	}
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func (app *Server) handleQuery(w http.ResponseWriter, req *http.Request) {
	tr := traffic.ParseTimeRange(req)
	qs := req.URL.Query().Get("q")
	if qs == "" {
		serve.Errorf(w, req, 400, "missing q parameter")
		return
	}
	q := traffic.ParseQuery(qs)
	data, err := app.model.QueryChartData(tr, q)
	if err != nil {
		serve.Errorf(w, req, 500, "query error: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (app *Server) handleValues(w http.ResponseWriter, req *http.Request) {
	tr := traffic.ParseTimeRange(req)
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
