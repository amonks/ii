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

	// Parse query params: use q= params if present, otherwise default.
	qParams := req.URL.Query()["q"]
	var queries []traffic.Query
	if len(qParams) > 0 {
		for _, qs := range qParams {
			queries = append(queries, traffic.ParseQuery(qs))
		}
	} else {
		queries = []traffic.Query{{Source: "stats", GroupBy: "host"}}
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

	// Run chart queries (one per q param).
	type queryResult struct {
		data map[string][]traffic.ChartPoint
		err  error
	}
	chartResults := make([]queryResult, len(queries))

	wg.Add(4 + len(queries))
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
	for i, q := range queries {
		go func(i int, q traffic.Query) {
			defer wg.Done()
			data, err := app.model.QueryChartData(tr, q)
			chartResults[i] = queryResult{data, err}
		}(i, q)
	}
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

	// Merge chart results: prefix series keys with query index.
	chartData := make(map[string][]traffic.ChartPoint)
	for i, cr := range chartResults {
		if cr.err != nil {
			log.Printf("serveTraffic: chartErr[%d]: %.200s", i, fmt.Sprint(cr.err))
			serve.Errorf(w, req, 500, "failed to run query %d: %s", i, cr.err)
			return
		}
		for key, pts := range cr.data {
			prefixed := fmt.Sprintf("q%d:%s", i, key)
			chartData[prefixed] = pts
		}
	}

	chartJSON, err := json.Marshal(chartData)
	if err != nil {
		serve.Errorf(w, req, 500, "failed to marshal chart data: %s", err)
		return
	}
	queriesJSON, err := json.Marshal(queries)
	if err != nil {
		serve.Errorf(w, req, 500, "failed to marshal queries: %s", err)
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
		QueriesJSON template.JS
	}

	pageData := PageData{
		TimeRange:   tr,
		Log:         logRows,
		TopPages:    topPages,
		StatusCodes: statusCodes,
		Durations:   durations,
		MaxDurCount: maxDurCount,
		ChartJSON:   template.JS(string(chartJSON)),
		QueriesJSON: template.JS(string(queriesJSON)),
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
	source := req.URL.Query().Get("source")
	dim := req.URL.Query().Get("dim")
	if source == "" || dim == "" {
		serve.Errorf(w, req, 400, "missing source or dim parameter")
		return
	}
	vals, err := app.model.GetDimensionValues(tr, source, dim)
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
