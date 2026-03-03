// Command builder is the CI builder process. It runs on ephemeral Fly
// machines, executes the CI pipeline, and reports results back to the
// orchestrator.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

func main() {
	if err := run(); err != nil {
		slog.Error("builder failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	reporter := NewReporter(cfg.OrchestratorURL, cfg.RunID)

	pipeline := &Pipeline{
		Config:   cfg,
		Reporter: reporter,
	}

	ctx := context.Background()

	status := "success"
	var pipelineErr error
	if err := pipeline.Run(ctx); err != nil {
		status = "failed"
		pipelineErr = err
		slog.Error("pipeline failed", "error", err)
	}

	// Report run completion to orchestrator.
	errMsg := ""
	if pipelineErr != nil {
		errMsg = pipelineErr.Error()
	}
	if err := reporter.FinishRun(status, errMsg); err != nil {
		slog.Error("failed to report run completion", "error", err)
	}

	if pipelineErr != nil {
		return pipelineErr
	}
	return nil
}

// Config holds environment-based configuration for the builder.
type Config struct {
	RunID           int64
	HeadSHA         string
	BaseSHA         string
	OrchestratorURL string
	FlyAPIToken     string
	GHToken         string
	Root            string
	DataDir         string
	BaseImageRef    string
}

func loadConfig() (*Config, error) {
	runIDStr := os.Getenv("CI_RUN_ID")
	if runIDStr == "" {
		return nil, fmt.Errorf("CI_RUN_ID not set")
	}
	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid CI_RUN_ID: %w", err)
	}

	cfg := &Config{
		RunID:           runID,
		HeadSHA:         requireEnv("CI_HEAD_SHA"),
		BaseSHA:         requireEnv("CI_BASE_SHA"),
		OrchestratorURL: requireEnv("CI_ORCHESTRATOR_URL"),
		FlyAPIToken:     os.Getenv("FLY_API_TOKEN"),
		GHToken:         os.Getenv("GH_TOKEN"),
		Root:            envOr("MONKS_ROOT", "/app"),
		DataDir:         envOr("MONKS_DATA", "/data"),
		BaseImageRef:    envOr("CI_BASE_IMAGE", "registry.fly.io/monks-ci-base:deployment-01KJS54RSKCKTZM364MVVDQWBG"),
	}
	return cfg, nil
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
