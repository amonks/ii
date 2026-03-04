package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// Reporter reports build progress to the orchestrator via HTTP.
type Reporter struct {
	baseURL string
	runID   int64
	client  *http.Client

	mu      sync.Mutex
	deploys []DeployResult
}

// NewReporter creates a reporter that talks to the orchestrator.
func NewReporter(baseURL string, runID int64, client *http.Client) *Reporter {
	return &Reporter{
		baseURL: baseURL,
		runID:   runID,
		client:  client,
	}
}

// StartJob tells the orchestrator a job has started.
func (r *Reporter) StartJob(name, kind string) error {
	body := map[string]string{"kind": kind}
	return r.put(fmt.Sprintf("/api/runs/%d/jobs/%s/start", r.runID, name), body)
}

// FinishJobResult contains the result of a finished job.
type FinishJobResult struct {
	Status     string
	DurationMs int64
	Error      string
	OutputPath string
}

// FinishJob tells the orchestrator a job has finished.
func (r *Reporter) FinishJob(name string, result FinishJobResult) error {
	body := map[string]any{
		"status":      result.Status,
		"duration_ms": result.DurationMs,
	}
	if result.Error != "" {
		body["error"] = result.Error
	}
	if result.OutputPath != "" {
		body["output_path"] = result.OutputPath
	}
	return r.put(fmt.Sprintf("/api/runs/%d/jobs/%s/done", r.runID, name), body)
}

// FinishStreamResult contains the result of a finished stream.
type FinishStreamResult struct {
	Status     string
	DurationMs int64
	Error      string
}

// StartStream tells the orchestrator a stream has started.
func (r *Reporter) StartStream(jobName, streamName string) error {
	return r.put(fmt.Sprintf("/api/runs/%d/jobs/%s/streams/%s/start", r.runID, jobName, streamName), nil)
}

// FinishStream tells the orchestrator a stream has finished.
func (r *Reporter) FinishStream(jobName, streamName string, result FinishStreamResult) error {
	body := map[string]any{
		"status":      result.Status,
		"duration_ms": result.DurationMs,
	}
	if result.Error != "" {
		body["error"] = result.Error
	}
	return r.put(fmt.Sprintf("/api/runs/%d/jobs/%s/streams/%s/done", r.runID, jobName, streamName), body)
}

// DeployResult contains deploy-specific result data for the task event.
type DeployResult struct {
	App         string `json:"app"`
	ImageRef    string `json:"image_ref,omitempty"`
	CompileMs   int64  `json:"compile_ms,omitempty"`
	PushMs      int64  `json:"push_ms,omitempty"`
	DeployMs    int64  `json:"deploy_ms,omitempty"`
	ImageBytes  int64  `json:"image_bytes,omitempty"`
	BinaryBytes int64  `json:"binary_bytes,omitempty"`
}

// AddDeployResult accumulates deploy metadata for the task event.
func (r *Reporter) AddDeployResult(d DeployResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deploys = append(r.deploys, d)
}

// RecordDeployment tells the orchestrator about a deployment.
func (r *Reporter) RecordDeployment(app, sha, imageRef string, binaryBytes int64) error {
	body := map[string]any{
		"app":          app,
		"commit_sha":   sha,
		"image_ref":    imageRef,
		"binary_bytes": binaryBytes,
	}
	return r.post(fmt.Sprintf("/api/runs/%d/deployments", r.runID), body)
}

// FinishRun tells the orchestrator the run is complete.
func (r *Reporter) FinishRun(status, errMsg string) error {
	body := map[string]any{
		"status": status,
	}
	if errMsg != "" {
		body["error"] = errMsg
	}
	r.mu.Lock()
	if len(r.deploys) > 0 {
		body["deploys"] = r.deploys
	}
	r.mu.Unlock()
	return r.put(fmt.Sprintf("/api/runs/%d/done", r.runID), body)
}

// StreamWriter returns an io.WriteCloser that streams output to the
// orchestrator for the given job and stream name.
func (r *Reporter) StreamWriter(jobName, stream string) *StreamWriter {
	return NewStreamWriter(r.client, r.baseURL, r.runID, jobName, stream)
}

// GetBaseSHA retrieves the base SHA for this run from the orchestrator.
func (r *Reporter) GetBaseSHA() (string, error) {
	url := r.baseURL + fmt.Sprintf("/api/runs/%d/base-sha", r.runID)
	resp, err := r.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("getting base SHA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("getting base SHA: status %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding base SHA response: %w", err)
	}
	return result["base_sha"], nil
}

func (r *Reporter) put(path string, body any) error {
	return r.doRequest(http.MethodPut, path, body)
}

func (r *Reporter) post(path string, body any) error {
	return r.doRequest(http.MethodPost, path, body)
}

func (r *Reporter) doRequest(method, path string, body any) error {
	bs, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := r.baseURL + path
	req, err := http.NewRequest(method, url, bytes.NewReader(bs))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	return nil
}
