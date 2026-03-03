package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"monks.co/pkg/flyapi"
	"monks.co/pkg/reqlog"
)

// TriggerHandler handles POST /trigger requests from GitHub Actions.
type TriggerHandler struct {
	model    *Model
	fly      *flyapi.Client
	builderConfig BuilderConfig
}

// BuilderConfig holds the configuration for creating builder machines.
type BuilderConfig struct {
	Image            string
	Region           string
	OrchestratorURL  string
	FlyAPIToken      string
	GHToken          string
	TSAuthKey        string
	AWSAccessKeyID   string
	AWSSecretKey     string
	AWSRegion        string
	GandiToken       string
}

type triggerRequest struct {
	SHA string `json:"sha"`
}

func (h *TriggerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req triggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SHA == "" {
		http.Error(w, "sha is required", http.StatusBadRequest)
		return
	}

	reqlog.Set(r.Context(), "trigger.sha", req.SHA)

	// Check if a run is already in progress.
	running, err := h.model.HasRunningRun()
	if err != nil {
		slog.Error("checking running runs", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if running {
		reqlog.Set(r.Context(), "trigger.skipped", "run_in_progress")
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, "run already in progress")
		return
	}

	// Determine base SHA from last successful run.
	baseSHA, err := h.model.LastSuccessfulSHA()
	if err != nil {
		// No previous successful run; use empty base (deploy all).
		baseSHA = "0000000000000000000000000000000000000000"
	}

	// Create the run.
	run, err := h.model.CreateRun(req.SHA, baseSHA, "webhook")
	if err != nil {
		slog.Error("creating run", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	reqlog.Set(r.Context(), "trigger.run_id", run.ID)
	reqlog.Set(r.Context(), "trigger.base_sha", baseSHA)

	// Create builder machine.
	if h.fly == nil {
		slog.Error("no fly client configured, cannot create builder machine", "run_id", run.ID)
		h.model.FinishRun(run.ID, "failed")
		http.Error(w, "fly client not configured (FLY_API_TOKEN missing?)", http.StatusInternalServerError)
		return
	}
	go h.createBuilderMachine(run)

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"run_id":   run.ID,
		"head_sha": req.SHA,
		"base_sha": baseSHA,
	})
}

func (h *TriggerHandler) createBuilderMachine(run *Run) {
	env := map[string]string{
		"CI_RUN_ID":           fmt.Sprintf("%d", run.ID),
		"CI_HEAD_SHA":         run.HeadSHA,
		"CI_BASE_SHA":         run.BaseSHA,
		"CI_ORCHESTRATOR_URL": h.builderConfig.OrchestratorURL,
		"FLY_API_TOKEN":       h.builderConfig.FlyAPIToken,
		"GH_TOKEN":            h.builderConfig.GHToken,
		"TS_AUTHKEY":          h.builderConfig.TSAuthKey,
	}
	if h.builderConfig.AWSAccessKeyID != "" {
		env["AWS_ACCESS_KEY_ID"] = h.builderConfig.AWSAccessKeyID
		env["AWS_SECRET_ACCESS_KEY"] = h.builderConfig.AWSSecretKey
		env["AWS_REGION"] = h.builderConfig.AWSRegion
	}
	if h.builderConfig.GandiToken != "" {
		env["GANDI_TOKEN"] = h.builderConfig.GandiToken
	}

	input := flyapi.MachineCreateInput{
		Name:   fmt.Sprintf("ci-builder-%d", run.ID),
		Region: h.builderConfig.Region,
		Config: flyapi.MachineConfig{
			Image: h.builderConfig.Image,
			Guest: flyapi.Guest{
				CPUKind:  "performance",
				CPUs:     4,
				MemoryMB: 4096,
			},
			Env:         env,
			AutoDestroy: true,
			Restart:     flyapi.RestartPolicy{Policy: "no"},
			Mounts: []flyapi.Mount{
				{Volume: "monks_ci_builder_cache", Path: "/data"},
			},
		},
	}

	info, err := h.fly.CreateMachine(nil, input)
	if err != nil {
		slog.Error("creating builder machine", "error", err, "run_id", run.ID)
		h.model.FinishRun(run.ID, "failed")
		return
	}

	slog.Info("created builder machine", "machine_id", info.ID, "run_id", run.ID)
	h.model.SetMachineID(run.ID, info.ID)
}
