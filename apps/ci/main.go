package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"monks.co/pkg/flyapi"
	"monks.co/pkg/gzip"
	"monks.co/pkg/meta"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

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

	// Dashboard routes.
	mux.HandleFunc("GET /", dashboardIndex(model))
	mux.HandleFunc("GET /runs/{id}", dashboardRun(model))
	mux.HandleFunc("GET /deployments", dashboardDeployments(model))
	mux.HandleFunc("GET /output/{runID}/{jobName}", serveOutput)

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
			Image:           envOr("CI_BUILDER_IMAGE", "registry.fly.io/monks-ci-builder:deployment-01KJS6B9YA7BYXTBRZV5TS4WSW"),
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
	RegisterAPI(mux, model, func(msg string) {
		sendSMS(msg)
	})

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

func dashboardRun(model *Model) http.HandlerFunc {
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

		w.Header().Set("Content-Type", "text/html")
		runPage(run, jobs).Render(r.Context(), w)
	}
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

func serveOutput(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("runID")
	jobName := r.PathValue("jobName")

	// Sanitize path components.
	outputPath := filepath.Join("/data/output/runs", runID, jobName+".log")
	outputPath = filepath.Clean(outputPath)

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		http.Error(w, "output not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	http.ServeFile(w, r, outputPath)
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
