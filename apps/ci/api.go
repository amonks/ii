package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
)

// RegisterAPI registers the builder callback API routes.
func RegisterAPI(mux *serve.Mux, model *Model, outputDir string, smsFunc func(string)) {
	api := &apiHandler{model: model, outputDir: outputDir, sendSMS: smsFunc}

	mux.HandleFunc("PUT /api/runs/{runID}/jobs/{name}/start", api.startJob)
	mux.HandleFunc("PUT /api/runs/{runID}/jobs/{name}/done", api.finishJob)
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

	if _, err := io.Copy(f, r.Body); err != nil {
		http.Error(w, fmt.Sprintf("writing output: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type finishJobRequest struct {
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	OutputPath string `json:"output_path,omitempty"`

	// Deploy-specific fields.
	Deploy *deployJobData `json:"deploy,omitempty"`

	// Terraform-specific fields.
	Terraform *terraformJobData `json:"terraform,omitempty"`
}

type deployJobData struct {
	App             string `json:"app"`
	ImageRef        string `json:"image_ref"`
	PreviousImage   string `json:"previous_image,omitempty"`
	BinaryBytes     int64  `json:"binary_bytes,omitempty"`
	ImageBytes      int64  `json:"image_bytes,omitempty"`
	CompileMs       int64  `json:"compile_ms,omitempty"`
	PushMs          int64  `json:"push_ms,omitempty"`
	DeployMs        int64  `json:"deploy_ms,omitempty"`
	PackagesChanged string `json:"packages_changed,omitempty"`
}

type terraformJobData struct {
	ResourcesAdded     int `json:"resources_added"`
	ResourcesChanged   int `json:"resources_changed"`
	ResourcesDestroyed int `json:"resources_destroyed"`
}

func (a *apiHandler) finishJob(w http.ResponseWriter, r *http.Request) {
	runID, err := a.parseRunID(r)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}
	name := r.PathValue("name")
	_ = runID // used for logging

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

	// Store kind-specific data.
	if req.Deploy != nil {
		bb := req.Deploy.BinaryBytes
		ib := req.Deploy.ImageBytes
		cm := req.Deploy.CompileMs
		pm := req.Deploy.PushMs
		dm := req.Deploy.DeployMs
		dj := &DeployJob{
			JobID:           jobID,
			App:             req.Deploy.App,
			ImageRef:        req.Deploy.ImageRef,
			BinaryBytes:     &bb,
			ImageBytes:      &ib,
			CompileMs:       &cm,
			PushMs:          &pm,
			DeployMs:        &dm,
			PackagesChanged: &req.Deploy.PackagesChanged,
		}
		if req.Deploy.PreviousImage != "" {
			dj.PreviousImage = &req.Deploy.PreviousImage
		}
		a.model.FinishDeployJob(dj)
	}

	if req.Terraform != nil {
		tj := &TerraformJob{
			JobID:              jobID,
			ResourcesAdded:     req.Terraform.ResourcesAdded,
			ResourcesChanged:   req.Terraform.ResourcesChanged,
			ResourcesDestroyed: req.Terraform.ResourcesDestroyed,
		}
		a.model.FinishTerraformJob(tj)
	}

	w.WriteHeader(http.StatusOK)
}

type finishRunRequest struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
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

	// Send SMS on failure.
	if req.Status == "failed" && a.sendSMS != nil {
		msg := fmt.Sprintf("CI run %d failed", runID)
		if req.Error != "" {
			msg += ": " + req.Error
		}
		a.sendSMS(msg)
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
