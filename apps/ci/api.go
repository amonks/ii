package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
)

// RegisterAPI registers the builder callback API routes.
func RegisterAPI(mux *serve.Mux, model *Model, outputDir string, smsFunc func(string), hub *OutputHub, trigger *TriggerHandler) {
	api := &apiHandler{model: model, outputDir: outputDir, sendSMS: smsFunc, hub: hub, trigger: trigger}

	mux.HandleFunc("PUT /api/runs/{runID}/jobs/{name}/start", api.startJob)
	mux.HandleFunc("PUT /api/runs/{runID}/jobs/{name}/done", api.finishJob)
	mux.HandleFunc("PUT /api/runs/{runID}/jobs/{name}/streams/{stream}/start", api.startStream)
	mux.HandleFunc("PUT /api/runs/{runID}/jobs/{name}/streams/{stream}/done", api.finishStream)
	mux.HandleFunc("POST /api/runs/{runID}/jobs/{name}/output/{stream}", api.appendOutput)
	mux.HandleFunc("PUT /api/runs/{runID}/done", api.finishRun)
	mux.HandleFunc("POST /runs/{runID}/mark-dead", api.markDead)
	mux.HandleFunc("GET /api/runs/{runID}/base-sha", api.getBaseSHA)
	mux.HandleFunc("POST /api/runs/{runID}/deployments", api.recordDeployment)
}

type apiHandler struct {
	model     *Model
	outputDir string
	sendSMS   func(string)
	hub       *OutputHub
	trigger   *TriggerHandler
}

func (a *apiHandler) parseRunID(r *http.Request) (int64, error) {
	s := r.PathValue("runID")
	return strconv.ParseInt(s, 10, 64)
}

type startJobRequest struct {
	Kind string `json:"kind"`
}

func (a *apiHandler) startJob(w http.ResponseWriter, r *http.Request) {
	runID, err := a.parseRunID(r)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	name := r.PathValue("name")

	var req startJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Kind = "unknown"
	}

	reqlog.Set(r.Context(), "api.run_id", runID)
	reqlog.Set(r.Context(), "api.job_name", name)

	outputPath := filepath.Join(a.outputDir, fmt.Sprintf("%d", runID), name)
	job, err := a.model.StartJob(runID, req.Kind, name, outputPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("starting job: %v", err), http.StatusInternalServerError)
		return
	}

	a.publishRunState(runID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"job_id": job.ID,
	})
}

func (a *apiHandler) appendOutput(w http.ResponseWriter, r *http.Request) {
	runID, err := a.parseRunID(r)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	name := r.PathValue("name")
	stream := r.PathValue("stream")

	dir := filepath.Join(a.outputDir, fmt.Sprintf("%d", runID), name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("creating output dir: %v", err), http.StatusInternalServerError)
		return
	}

	f, err := os.OpenFile(filepath.Join(dir, stream+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, fmt.Sprintf("opening output file: %v", err), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("reading body: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err := f.Write(body); err != nil {
		http.Error(w, fmt.Sprintf("writing output: %v", err), http.StatusInternalServerError)
		return
	}

	if a.hub != nil {
		key := fmt.Sprintf("%d/%s/%s", runID, name, stream)
		a.hub.Publish(key, body)

		// On first write to a new stream, publish updated run state.
		streamKey := fmt.Sprintf("%d/%s/%s", runID, name, stream)
		if _, loaded := knownStreams.LoadOrStore(streamKey, true); !loaded {
			a.publishRunState(runID)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

type finishJobRequest struct {
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	OutputPath string `json:"output_path,omitempty"`
}

func (a *apiHandler) finishJob(w http.ResponseWriter, r *http.Request) {
	runID, err := a.parseRunID(r)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	name := r.PathValue("name")

	reqlog.Set(r.Context(), "api.run_id", runID)
	reqlog.Set(r.Context(), "api.job_name", name)

	var req finishJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Find the job by run_id and name.
	_, jobs, err := a.model.RunWithJobs(runID)
	if err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	var jobID int64
	for _, j := range jobs {
		if j.Name == name {
			jobID = j.ID
			break
		}
	}
	if jobID == 0 {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	if err := a.model.FinishJob(jobID, req.Status, req.DurationMs, req.Error, req.OutputPath); err != nil {
		http.Error(w, fmt.Sprintf("finishing job: %v", err), http.StatusInternalServerError)
		return
	}

	if a.hub != nil {
		prefix := fmt.Sprintf("%d/%s/", runID, name)
		a.hub.CloseAll(prefix)
	}

	a.publishRunState(runID)

	w.WriteHeader(http.StatusOK)
}

func (a *apiHandler) startStream(w http.ResponseWriter, r *http.Request) {
	runID, err := a.parseRunID(r)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	jobName := r.PathValue("name")
	streamName := r.PathValue("stream")

	reqlog.Set(r.Context(), "api.run_id", runID)
	reqlog.Set(r.Context(), "api.job_name", jobName)
	reqlog.Set(r.Context(), "api.stream_name", streamName)

	// Find the job by run_id and name.
	_, jobs, err := a.model.RunWithJobs(runID)
	if err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	var jobID int64
	for _, j := range jobs {
		if j.Name == jobName {
			jobID = j.ID
			break
		}
	}
	if jobID == 0 {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	s, err := a.model.StartStream(jobID, streamName)
	if err != nil {
		http.Error(w, fmt.Sprintf("starting stream: %v", err), http.StatusInternalServerError)
		return
	}

	a.publishRunState(runID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"stream_id": s.ID,
	})
}

type finishStreamRequest struct {
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

func (a *apiHandler) finishStream(w http.ResponseWriter, r *http.Request) {
	runID, err := a.parseRunID(r)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	jobName := r.PathValue("name")
	streamName := r.PathValue("stream")

	reqlog.Set(r.Context(), "api.run_id", runID)
	reqlog.Set(r.Context(), "api.job_name", jobName)
	reqlog.Set(r.Context(), "api.stream_name", streamName)

	var req finishStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Find the job, then the stream.
	_, jobs, err := a.model.RunWithJobs(runID)
	if err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	var jobID int64
	for _, j := range jobs {
		if j.Name == jobName {
			jobID = j.ID
			break
		}
	}
	if jobID == 0 {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	streams, err := a.model.StreamsForJob(jobID)
	if err != nil {
		http.Error(w, "error loading streams", http.StatusInternalServerError)
		return
	}

	var streamID int64
	for _, s := range streams {
		if s.Name == streamName {
			streamID = s.ID
			break
		}
	}
	if streamID == 0 {
		http.Error(w, "stream not found", http.StatusNotFound)
		return
	}

	if err := a.model.FinishStream(streamID, req.Status, req.DurationMs, req.Error); err != nil {
		http.Error(w, fmt.Sprintf("finishing stream: %v", err), http.StatusInternalServerError)
		return
	}

	a.publishRunState(runID)

	w.WriteHeader(http.StatusOK)
}

type finishRunRequest struct {
	Status  string            `json:"status"`
	Error   string            `json:"error,omitempty"`
	Deploys []deployEventData `json:"deploys,omitempty"`
}

// deployEventData carries deploy metadata for the task event log.
type deployEventData struct {
	App         string `json:"app"`
	ImageRef    string `json:"image_ref,omitempty"`
	CompileMs   int64  `json:"compile_ms,omitempty"`
	PushMs      int64  `json:"push_ms,omitempty"`
	DeployMs    int64  `json:"deploy_ms,omitempty"`
	ImageBytes  int64  `json:"image_bytes,omitempty"`
	BinaryBytes int64  `json:"binary_bytes,omitempty"`
}

func (a *apiHandler) finishRun(w http.ResponseWriter, r *http.Request) {
	runID, err := a.parseRunID(r)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	reqlog.Set(r.Context(), "api.run_id", runID)

	var req finishRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := a.model.FinishRun(runID, req.Status, req.Error); err != nil {
		http.Error(w, fmt.Sprintf("finishing run: %v", err), http.StatusInternalServerError)
		return
	}

	a.publishRunState(runID)
	a.closeRunEvents(runID)

	// Emit a wide "task" event with all run metadata.
	a.emitTaskEvent(runID, req.Status, req.Error, req.Deploys)

	// Send SMS on completion.
	if a.sendSMS != nil {
		var msg string
		link := fmt.Sprintf("https://monks.co/ci/runs/%d", runID)
		switch req.Status {
		case "failed":
			msg = fmt.Sprintf("CI run %d failed: %s", runID, link)
			if req.Error != "" {
				msg += "\n" + req.Error
			}
		case "success":
			msg = fmt.Sprintf("CI run %d succeeded: %s", runID, link)
		}
		if msg != "" {
			a.sendSMS(msg)
		}
	}

	// Check for pending trigger and start a new build if one exists.
	if a.trigger != nil {
		go a.trigger.StartPendingBuild()
	}

	w.WriteHeader(http.StatusOK)
}

func (a *apiHandler) markDead(w http.ResponseWriter, r *http.Request) {
	runID, err := a.parseRunID(r)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	run, _, err := a.model.RunWithJobs(runID)
	if err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	if run.Status != "running" {
		http.Error(w, "can only mark running runs as dead", http.StatusBadRequest)
		return
	}

	if err := a.model.FinishRun(runID, "dead", "manually marked as dead"); err != nil {
		http.Error(w, fmt.Sprintf("marking run dead: %v", err), http.StatusInternalServerError)
		return
	}

	a.publishRunState(runID)
	a.closeRunEvents(runID)

	http.Redirect(w, r, fmt.Sprintf("runs/%d", runID), http.StatusFound)
}

func (a *apiHandler) getBaseSHA(w http.ResponseWriter, r *http.Request) {
	runID, err := a.parseRunID(r)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	run, _, err := a.model.RunWithJobs(runID)
	if err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"base_sha": run.BaseSHA,
	})
}

type recordDeploymentRequest struct {
	App         string `json:"app"`
	CommitSHA   string `json:"commit_sha"`
	ImageRef    string `json:"image_ref"`
	BinaryBytes int64  `json:"binary_bytes,omitempty"`
}

func (a *apiHandler) recordDeployment(w http.ResponseWriter, r *http.Request) {
	runID, err := a.parseRunID(r)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	_ = runID

	var req recordDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	d := &Deployment{
		App:       req.App,
		CommitSHA: req.CommitSHA,
		ImageRef:  req.ImageRef,
	}
	if req.BinaryBytes > 0 {
		bb := req.BinaryBytes
		d.BinaryBytes = &bb
	}

	if err := a.model.RecordDeployment(d); err != nil {
		http.Error(w, fmt.Sprintf("recording deployment: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// emitTaskEvent logs a wide "task" event with all run metadata flattened into dotted keys.
func (a *apiHandler) emitTaskEvent(runID int64, status, errorMsg string, deploys []deployEventData) {
	run, jobs, err := a.model.RunWithJobs(runID)
	if err != nil {
		slog.Warn("emitTaskEvent: failed to load run", "error", err, "run_id", runID)
		return
	}

	// Compute run duration from timestamps.
	var durationMs int64
	if d := durationFromTimestamps(&run.StartedAt, run.FinishedAt); d != nil {
		durationMs = *d
	}

	attrs := []any{
		"task.name", "ci-run",
		"task.status", status,
		"task.duration_ms", durationMs,
		"run.id", runID,
		"run.head_sha", run.HeadSHA,
		"run.base_sha", run.BaseSHA,
		"run.trigger", run.Trigger,
	}
	if errorMsg != "" {
		attrs = append(attrs, "task.error", errorMsg)
	}

	// Per-job keys for finished jobs.
	for _, j := range jobs {
		if j.Status == "pending" || j.Status == "in_progress" {
			continue
		}
		prefix := "job." + j.Name
		attrs = append(attrs, prefix+".status", j.Status)
		if j.DurationMs != nil {
			attrs = append(attrs, prefix+".duration_ms", *j.DurationMs)
		}
	}

	// Per-stream keys from DB.
	streams, _ := a.model.StreamsForRun(runID)
	// Build job ID -> name map.
	jobNameByID := make(map[int64]string, len(jobs))
	for _, j := range jobs {
		jobNameByID[j.ID] = j.Name
	}
	for _, s := range streams {
		jobName := jobNameByID[s.JobID]
		prefix := "stream." + jobName + "." + s.Name
		attrs = append(attrs, prefix+".status", s.Status)
		if s.DurationMs != nil {
			attrs = append(attrs, prefix+".duration_ms", *s.DurationMs)
		}
	}

	// Deploy-specific keys from request payload.
	for _, d := range deploys {
		prefix := "deploy." + d.App
		if d.ImageRef != "" {
			attrs = append(attrs, prefix+".image_ref", d.ImageRef)
		}
		if d.CompileMs > 0 {
			attrs = append(attrs, prefix+".compile_ms", d.CompileMs)
		}
		if d.PushMs > 0 {
			attrs = append(attrs, prefix+".push_ms", d.PushMs)
		}
		if d.DeployMs > 0 {
			attrs = append(attrs, prefix+".deploy_ms", d.DeployMs)
		}
		if d.ImageBytes > 0 {
			attrs = append(attrs, prefix+".image_bytes", d.ImageBytes)
		}
	}

	slog.Info("task", attrs...)
}
