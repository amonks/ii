package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"monks.co/pkg/flyapi"
	"monks.co/pkg/gzip"
	"monks.co/pkg/meta"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

//go:embed static
var staticFS embed.FS

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("fatal", "error", err.Error(), "app.name", meta.AppName())
		}
		reqlog.Shutdown()
		os.Exit(1)
	}
}

func run() error {
	reqlog.SetupLogging()

	model, err := NewModel()
	if err != nil {
		return fmt.Errorf("initializing model: %w", err)
	}

	mux := serve.NewMux()
	hub := NewOutputHub()

	outputDir := filepath.Join(envOr("MONKS_DATA", "/data"), "output", "runs")

	// Dashboard routes.
	mux.HandleFunc("GET /", dashboardIndex(model))
	mux.HandleFunc("GET /runs/{id}", dashboardRun(model, outputDir))
	mux.HandleFunc("GET /runs/{id}/events", sseHandler(model, outputDir, hub))
	mux.HandleFunc("GET /deployments", dashboardDeployments(model))
	mux.HandleFunc("GET /output/{runID}/{jobName}", serveJobStreams(outputDir))
	mux.HandleFunc("GET /output/{runID}/{jobName}/{stream}", serveStream(outputDir, hub))
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticSub)))

	// Trigger endpoint.
	flyToken := os.Getenv("FLY_API_TOKEN")
	var flyClient *flyapi.Client
	if flyToken != "" {
		flyClient = flyapi.NewClient(flyToken, "monks-ci-builder")
	}

	trigger := &TriggerHandler{
		model: model,
		fly:   flyClient,
		builderConfig: BuilderConfig{
			FallbackImage:   "registry.fly.io/monks-ci-builder:deployment-01KJS6B9YA7BYXTBRZV5TS4WSW",
			Region:          "ord",
			OrchestratorURL: "http://monks-ci-fly-ord",
			FlyAPIToken:     flyToken,
			GHToken:         os.Getenv("GH_TOKEN"),
			TSAuthKey:       os.Getenv("TS_AUTHKEY"),
			AWSAccessKeyID:  os.Getenv("AWS_ACCESS_KEY_ID"),
			AWSSecretKey:    os.Getenv("AWS_SECRET_ACCESS_KEY"),
			AWSRegion:       os.Getenv("AWS_REGION"),
			GandiToken:      os.Getenv("GANDI_TOKEN"),
		},
	}
	mux.Handle("POST /trigger", trigger)

	// Builder callback API.
	RegisterAPI(mux, model, outputDir, func(msg string) {
		sendSMS(msg)
	}, hub, trigger)

	ctx := sigctx.New()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}
	if err := tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(mux))); err != nil {
		return err
	}

	return nil
}

func dashboardIndex(model *Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runs, err := model.RecentRuns(20)
		if err != nil {
			http.Error(w, "error loading runs", http.StatusInternalServerError)
			return
		}

		deployments, err := model.CurrentDeployments()
		if err != nil {
			http.Error(w, "error loading deployments", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		indexPage(runs, deployments).Render(r.Context(), w)
	}
}

// StreamInfo holds metadata about a single output stream for template rendering.
type StreamInfo struct {
	Name        string // URL-safe name (~ instead of /)
	DisplayName string // human-readable name (/ restored)
	Status      string
	DurationMs  *int64
	Error       *string
	LastLine    string
}

// decodeStreamName restores "/" from "~" in encoded stream names.
func decodeStreamName(name string) string {
	return strings.ReplaceAll(name, "~", "/")
}

func dashboardRun(model *Model, outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid run ID", http.StatusBadRequest)
			return
		}

		run, jobs, err := model.RunWithJobs(id)
		if err != nil {
			http.Error(w, "run not found", http.StatusNotFound)
			return
		}

		// Load streams from DB and collect output metadata.
		dbStreams, _ := model.StreamsForRun(id)

		// Index by job ID.
		streamsByJobID := make(map[int64][]Stream)
		for _, s := range dbStreams {
			streamsByJobID[s.JobID] = append(streamsByJobID[s.JobID], s)
		}

		// Build job ID -> name map.
		jobNameByID := make(map[int64]string, len(jobs))
		for _, j := range jobs {
			jobNameByID[j.ID] = j.Name
		}

		streams := map[string][]StreamInfo{}
		for _, j := range jobs {
			if jobStreams, ok := streamsByJobID[j.ID]; ok {
				for _, s := range jobStreams {
					dir := filepath.Join(outputDir, idStr, j.Name)
					lastLine := readLastLine(filepath.Join(dir, s.Name+".log"))
					streams[j.Name] = append(streams[j.Name], StreamInfo{
						Name:        s.Name,
						DisplayName: decodeStreamName(s.Name),
						Status:      s.Status,
						DurationMs:  s.DurationMs,
						Error:       s.Error,
						LastLine:    lastLine,
					})
				}
				continue
			}

			// Fallback: scan output directory for streams (legacy data).
			if j.OutputPath == nil {
				continue
			}
			dir := filepath.Join(outputDir, idStr, j.Name)
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				name := strings.TrimSuffix(e.Name(), ".log")
				lastLine := readLastLine(filepath.Join(dir, e.Name()))
				streams[j.Name] = append(streams[j.Name], StreamInfo{
					Name:        name,
					DisplayName: decodeStreamName(name),
					Status:      j.Status,
					LastLine:    lastLine,
				})
			}
		}

		var logs []LogEvent
		if fetchedLogs, err := FetchRunLogs(run); err != nil {
			slog.Warn("failed to fetch run logs", "error", err, "run_id", id)
		} else {
			logs = fetchedLogs
		}

		w.Header().Set("Content-Type", "text/html")
		runPage(run, jobs, streams, logs).Render(r.Context(), w)
	}
}

// readLastLine returns the last non-empty line of a file, truncated to 120 chars.
func readLastLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	s := strings.TrimRight(string(data), "\n")
	idx := strings.LastIndex(s, "\n")
	line := s
	if idx >= 0 {
		line = s[idx+1:]
	}
	if utf8.RuneCountInString(line) > 120 {
		runes := []rune(line)
		line = string(runes[:120]) + "..."
	}
	return line
}

func dashboardDeployments(model *Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get all deployments, not just current.
		var deployments []Deployment
		model.db.Order("id DESC").Limit(100).Find(&deployments)

		w.Header().Set("Content-Type", "text/html")
		deploymentsPage(deployments).Render(r.Context(), w)
	}
}

func serveJobStreams(outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID := r.PathValue("runID")
		jobName := r.PathValue("jobName")

		dir := filepath.Clean(filepath.Join(outputDir, runID, jobName))
		entries, err := os.ReadDir(dir)
		if err != nil {
			http.Error(w, "output not found", http.StatusNotFound)
			return
		}

		// If there's exactly one stream, redirect directly to it.
		if len(entries) == 1 {
			name := strings.TrimSuffix(entries[0].Name(), ".log")
			http.Redirect(w, r, fmt.Sprintf("%s/%s", r.URL.Path, name), http.StatusFound)
			return
		}

		// List streams as plain text with links.
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<h2>Streams for %s</h2><ul>", jobName)
		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".log")
			fmt.Fprintf(w, `<li><a href="%s/%s">%s</a></li>`, r.URL.Path, name, decodeStreamName(name))
		}
		fmt.Fprintf(w, "</ul>")
	}
}

func serveStream(outputDir string, hub *OutputHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID := r.PathValue("runID")
		jobName := r.PathValue("jobName")
		stream := r.PathValue("stream")

		filePath := filepath.Clean(filepath.Join(outputDir, runID, jobName, stream+".log"))

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Write existing file content.
		if data, err := os.ReadFile(filePath); err == nil && len(data) > 0 {
			w.Write(data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		// If not streaming requested, return now.
		if r.URL.Query().Get("stream") != "1" {
			return
		}

		// Subscribe to live updates.
		key := fmt.Sprintf("%s/%s/%s", runID, jobName, stream)
		ch, unsub := hub.Subscribe(key)
		defer unsub()

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		ctx := r.Context()
		for {
			select {
			case data, ok := <-ch:
				if !ok {
					return
				}
				w.Write(data)
				flusher.Flush()
			case <-ctx.Done():
				return
			}
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func sendSMS(message string) {
	client := tailnet.Client()
	if client == nil {
		slog.Warn("no tailnet client for SMS")
		return
	}
	smsURL := "http://monks-sms-brigid/?message=" + url.QueryEscape(message)
	resp, err := client.Post(smsURL, "text/plain", nil)
	if err != nil {
		slog.Error("sending SMS", "error", err)
		return
	}
	resp.Body.Close()
}
