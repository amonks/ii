package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"monks.co/pkg/flyapi"
	"monks.co/pkg/reqlog"
)

// TriggerHandler handles POST /trigger requests from GitHub Actions.
type TriggerHandler struct {
	model         *Model
	fly           *flyapi.Client
	builderConfig BuilderConfig

	// destroyTimeout is how long to wait for an old builder to be
	// destroyed before giving up. Defaults to 1 minute.
	destroyTimeout time.Duration
}

// BuilderConfig holds the configuration for creating builder machines.
type BuilderConfig struct {
	FallbackImage         string
	Region                string
	OrchestratorURL       string
	FlyAPIToken           string
	GHToken               string
	TSAuthKey             string
	AWSAccessKeyID        string
	AWSSecretKey          string
	AWSRegion             string
	GandiToken            string
	TSOAuthClientID       string
	TSOAuthClientSecret   string
	TSTailnetID           string
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
	run, currentJob, err := h.model.RunningRun()
	if err != nil {
		// No running run — start a new one.
		h.startNewRun(w, r, req.SHA)
		return
	}

	// A run is in progress. Behavior depends on the current phase.
	reqlog.Set(r.Context(), "trigger.superseding_run", run.ID)
	reqlog.Set(r.Context(), "trigger.current_job", currentJob)

	// If the run is stuck in "restarting" for too long (e.g. the
	// orchestrator crashed after setting the status but before creating
	// a continuation builder), fail it and start fresh.
	if run.Status == "restarting" {
		startedAt, err := time.Parse(time.RFC3339, run.StartedAt)
		if err == nil && time.Since(startedAt) > 15*time.Minute {
			slog.Warn("failing stale restarting run", "run_id", run.ID, "started_at", run.StartedAt)
			reqlog.Set(r.Context(), "trigger.action", "fail_stale_restart")
			h.model.FinishRun(run.ID, "failed", "stuck in restarting state")
			h.startNewRun(w, r, req.SHA)
			return
		}
	}

	if currentJob == "deploy" || run.Status == "restarting" {
		// During deploy or restarting: don't interrupt, just record the
		// pending SHA. It will be picked up when the run finishes.
		if err := h.model.SetPendingTrigger(req.SHA); err != nil {
			slog.Error("setting pending trigger", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		reqlog.Set(r.Context(), "trigger.action", "queued_pending")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "queued",
			"message": "deploy in progress; build will start when it finishes",
			"sha":     req.SHA,
		})
		return
	}

	// During fetch/test/generate: kill the builder, mark run superseded, start new.
	reqlog.Set(r.Context(), "trigger.action", "supersede")

	// Mark old run superseded immediately.
	if err := h.model.FinishRun(run.ID, "superseded", "superseded by newer commit"); err != nil {
		slog.Error("marking run superseded", "error", err, "run_id", run.ID)
	}

	// Create the new run and respond to the webhook right away.
	baseSHA, err := h.model.LastSuccessfulSHA()
	if err != nil {
		baseSHA = "0000000000000000000000000000000000000000"
	}
	newRun, err := h.model.CreateRun(req.SHA, baseSHA, "webhook")
	if err != nil {
		slog.Error("creating run", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	reqlog.Set(r.Context(), "trigger.run_id", newRun.ID)
	reqlog.Set(r.Context(), "trigger.base_sha", baseSHA)
	if h.fly == nil {
		slog.Error("no fly client configured, cannot create builder machine", "run_id", newRun.ID)
		h.model.FinishRun(newRun.ID, "failed", "fly client not configured (FLY_API_TOKEN missing?)")
		http.Error(w, "fly client not configured (FLY_API_TOKEN missing?)", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"run_id":   newRun.ID,
		"head_sha": req.SHA,
		"base_sha": baseSHA,
	})

	// In the background: stop the old builder, wait for it to be destroyed,
	// then create the new builder. This ensures the shared volume is released.
	prevMachineID := ""
	if run.MachineID != nil {
		prevMachineID = *run.MachineID
	}
	go h.supersedeAndStart(prevMachineID, newRun)
}

// BuildNow handles POST /build requests from the dashboard.
// It triggers a manual build of "main" and redirects to the new run's page.
func (h *TriggerHandler) BuildNow(w http.ResponseWriter, r *http.Request) {
	reqlog.Set(r.Context(), "trigger.sha", "main")
	reqlog.Set(r.Context(), "trigger.action", "manual")

	// Check if a run is already in progress.
	if _, _, err := h.model.RunningRun(); err == nil {
		http.Error(w, "a build is already running", http.StatusConflict)
		return
	}

	baseSHA, err := h.model.LastSuccessfulSHA()
	if err != nil {
		baseSHA = "0000000000000000000000000000000000000000"
	}

	run, err := h.model.CreateRun("main", baseSHA, "manual")
	if err != nil {
		slog.Error("creating manual run", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	reqlog.Set(r.Context(), "trigger.run_id", run.ID)

	if h.fly == nil {
		slog.Error("no fly client configured", "run_id", run.ID)
		h.model.FinishRun(run.ID, "failed", "fly client not configured")
		http.Error(w, "fly client not configured", http.StatusInternalServerError)
		return
	}
	go h.createBuilderMachine(run)

	http.Redirect(w, r, fmt.Sprintf("runs/%d", run.ID), http.StatusSeeOther)
}

// supersedeAndStart stops the old builder machine, waits for it to be
// destroyed (so the shared volume is released), then creates the new builder.
// If the old machine doesn't die within 1 minute, the new run is failed.
func (h *TriggerHandler) supersedeAndStart(prevMachineID string, newRun *Run) {
	timeout := h.destroyTimeout
	if timeout == 0 {
		timeout = time.Minute
	}
	if prevMachineID != "" {
		if err := h.stopAndWaitForDestroyed(prevMachineID, timeout); err != nil {
			slog.Error("old builder did not die in time", "error", err, "machine_id", prevMachineID, "run_id", newRun.ID)
			h.model.FinishRun(newRun.ID, "failed", fmt.Sprintf("old builder did not die in time: %v", err))
			return
		}
	}
	h.createBuilderMachine(newRun)
}

// stopAndWaitForDestroyed stops a machine and waits for it to reach the
// "destroyed" state. It retries the stop+wait cycle up to 3 times, with an
// overall deadline of the given timeout.
func (h *TriggerHandler) stopAndWaitForDestroyed(machineID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for attempt := 1; ; attempt++ {
		slog.Info("stopping old builder", "machine_id", machineID, "attempt", attempt)
		if err := h.fly.StopMachine(ctx, machineID); err != nil {
			slog.Warn("stopping builder machine", "error", err, "machine_id", machineID, "attempt", attempt)
		}

		deadline, _ := ctx.Deadline()
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timeout waiting for machine %s to be destroyed", machineID)
		}

		err := h.fly.WaitForState(ctx, machineID, "destroyed", remaining)
		if err == nil {
			slog.Info("old builder destroyed", "machine_id", machineID)
			return nil
		}

		slog.Warn("waiting for builder destruction", "error", err, "machine_id", machineID, "attempt", attempt)

		if ctx.Err() != nil {
			return fmt.Errorf("timeout waiting for machine %s to be destroyed after %d attempts", machineID, attempt)
		}
	}
}

func (h *TriggerHandler) startNewRun(w http.ResponseWriter, r *http.Request, sha string, trigger ...string) {
	trig := "webhook"
	if len(trigger) > 0 {
		trig = trigger[0]
	}
	// Determine base SHA from last successful run.
	baseSHA, err := h.model.LastSuccessfulSHA()
	if err != nil {
		// No previous successful run; use empty base (deploy all).
		baseSHA = "0000000000000000000000000000000000000000"
	}

	// Create the run.
	run, err := h.model.CreateRun(sha, baseSHA, trig)
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
		h.model.FinishRun(run.ID, "failed", "fly client not configured (FLY_API_TOKEN missing?)")
		http.Error(w, "fly client not configured (FLY_API_TOKEN missing?)", http.StatusInternalServerError)
		return
	}
	go h.createBuilderMachine(run)

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"run_id":   run.ID,
		"head_sha": sha,
		"base_sha": baseSHA,
	})
}

// StartPendingBuild checks for a pending trigger and starts a new build if one exists.
// If prevMachineID is non-empty, it waits for that machine to be destroyed before
// creating the new builder, so that the shared volume is released.
func (h *TriggerHandler) StartPendingBuild(prevMachineID string) {
	sha, ok, err := h.model.PopPendingTrigger()
	if err != nil {
		slog.Error("popping pending trigger", "error", err)
		return
	}
	if !ok {
		return
	}

	slog.Info("starting pending build", "sha", sha)

	// Wait for the previous builder machine to be fully destroyed so the
	// shared volume (monks_ci_builder_cache) is detached and available.
	if prevMachineID != "" && h.fly != nil {
		slog.Info("waiting for previous builder to be destroyed", "machine_id", prevMachineID)
		if err := h.fly.WaitForState(context.Background(), prevMachineID, "destroyed", 5*time.Minute); err != nil {
			slog.Warn("waiting for previous builder destruction", "error", err, "machine_id", prevMachineID)
			// Continue anyway — the volume may have been released.
		}
	}

	baseSHA, err := h.model.LastSuccessfulSHA()
	if err != nil {
		baseSHA = "0000000000000000000000000000000000000000"
	}

	run, err := h.model.CreateRun(sha, baseSHA, "pending")
	if err != nil {
		slog.Error("creating pending run", "error", err)
		return
	}

	if h.fly == nil {
		slog.Error("no fly client for pending build", "run_id", run.ID)
		h.model.FinishRun(run.ID, "failed", "fly client not configured")
		return
	}

	h.createBuilderMachine(run)
}

func (h *TriggerHandler) resolveBuilderImage() string {
	image, err := h.fly.LatestImage(context.Background())
	if err != nil {
		slog.Warn("failed to resolve builder image from registry, using fallback", "error", err)
		return h.builderConfig.FallbackImage
	}
	slog.Info("resolved builder image from registry", "image", image)
	return image
}

// ContinueRun waits for the old builder machine to die and creates a
// continuation builder for the given run in its current phase.
func (h *TriggerHandler) ContinueRun(run *Run, prevMachineID string) {
	if prevMachineID != "" && h.fly != nil {
		slog.Info("waiting for previous builder to be destroyed", "machine_id", prevMachineID, "run_id", run.ID)
		if err := h.fly.WaitForState(context.Background(), prevMachineID, "destroyed", 5*time.Minute); err != nil {
			slog.Warn("waiting for previous builder destruction", "error", err, "machine_id", prevMachineID)
		}
	}

	// Set status back to running now that we're creating a new builder.
	if err := h.model.UpdateRunPhase(run.ID, run.Phase, "running"); err != nil {
		slog.Error("updating run phase for continuation", "error", err, "run_id", run.ID)
	}

	h.createBuilderMachine(run)
}

func (h *TriggerHandler) createBuilderMachine(run *Run) {
	phase := run.Phase
	if phase == "" {
		phase = "initial"
	}

	env := map[string]string{
		"CI_RUN_ID":           fmt.Sprintf("%d", run.ID),
		"CI_HEAD_SHA":         run.HeadSHA,
		"CI_BASE_SHA":         run.BaseSHA,
		"CI_ORCHESTRATOR_URL": h.builderConfig.OrchestratorURL,
		"CI_PHASE":            phase,
		"FLY_API_TOKEN":       h.builderConfig.FlyAPIToken,
		"GH_TOKEN":            h.builderConfig.GHToken,
		"TS_AUTHKEY":          h.builderConfig.TSAuthKey,
		"MONKS_APP_NAME":      "ci-builder",
	}
	if h.builderConfig.AWSAccessKeyID != "" {
		env["AWS_ACCESS_KEY_ID"] = h.builderConfig.AWSAccessKeyID
		env["AWS_SECRET_ACCESS_KEY"] = h.builderConfig.AWSSecretKey
		env["AWS_REGION"] = h.builderConfig.AWSRegion
	}
	if h.builderConfig.GandiToken != "" {
		env["GANDI_TOKEN"] = h.builderConfig.GandiToken
	}
	if h.builderConfig.TSOAuthClientID != "" {
		env["TAILSCALE_OAUTH_CLIENT_ID"] = h.builderConfig.TSOAuthClientID
		env["TAILSCALE_OAUTH_CLIENT_SECRET"] = h.builderConfig.TSOAuthClientSecret
		env["TAILSCALE_TAILNET_ID"] = h.builderConfig.TSTailnetID
	}

	input := flyapi.MachineCreateInput{
		Name:   fmt.Sprintf("ci-builder-%d", run.ID),
		Region: h.builderConfig.Region,
		Config: flyapi.MachineConfig{
			Image: h.resolveBuilderImage(),
			Guest: flyapi.Guest{
				CPUKind:  "performance",
				CPUs:     4,
				MemoryMB: 8192,
			},
			Env:         env,
			AutoDestroy: true,
			Restart:     flyapi.RestartPolicy{Policy: "no"},
			Mounts: []flyapi.Mount{
				{Volume: "monks_ci_builder_cache", Path: "/data"},
			},
		},
	}

	info, err := h.fly.CreateMachine(context.Background(), input)
	if err != nil {
		slog.Error("creating builder machine", "error", err, "run_id", run.ID)
		h.model.FinishRun(run.ID, "failed", fmt.Sprintf("creating builder machine: %v", err))
		return
	}

	slog.Info("created builder machine", "machine_id", info.ID, "run_id", run.ID)
	h.model.SetMachineID(run.ID, info.ID)
}
