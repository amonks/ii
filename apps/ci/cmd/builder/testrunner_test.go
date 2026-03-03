package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordingHandler struct {
	mu       sync.Mutex
	requests []recordedRequest
	// output accumulates data received at output streaming endpoints, keyed by path.
	output map[string][]byte
}

type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

func newRecordingHandler() *recordingHandler {
	return &recordingHandler{output: make(map[string][]byte)}
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Handle output streaming POSTs.
	if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/output/") {
		data, _ := io.ReadAll(r.Body)
		h.output[r.URL.Path] = append(h.output[r.URL.Path], data...)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)

	h.requests = append(h.requests, recordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Body:   body,
	})

	// Return job_id for start requests.
	if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/start") {
		json.NewEncoder(w).Encode(map[string]any{"job_id": 1})
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *recordingHandler) getRequests() []recordedRequest {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]recordedRequest, len(h.requests))
	copy(result, h.requests)
	return result
}

func (h *recordingHandler) getOutput(path string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return string(h.output[path])
}

func (h *recordingHandler) outputPaths() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var paths []string
	for p := range h.output {
		paths = append(paths, p)
	}
	return paths
}

func TestReporterIntegration(t *testing.T) {
	handler := newRecordingHandler()
	srv := httptest.NewServer(handler)
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1)

	// Start a job.
	if err := reporter.StartJob("go-test", "test"); err != nil {
		t.Fatal(err)
	}

	// Finish a job.
	if err := reporter.FinishJob("go-test", FinishJobResult{
		Status:     "success",
		DurationMs: 500,
	}); err != nil {
		t.Fatal(err)
	}

	reqs := handler.getRequests()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(reqs))
	}

	if reqs[0].Path != "/api/runs/1/jobs/go-test/start" {
		t.Errorf("first request path = %s", reqs[0].Path)
	}
	if reqs[1].Path != "/api/runs/1/jobs/go-test/done" {
		t.Errorf("second request path = %s", reqs[1].Path)
	}
}

func TestRunTestsStreamsPerTaskOutput(t *testing.T) {
	handler := newRecordingHandler()
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Create a temp dir with a simple tasks.toml.
	dir := t.TempDir()
	tasksToml := `
[[task]]
id = "hello"
type = "short"
cmd = "echo hello from task"
`
	os.WriteFile(filepath.Join(dir, "tasks.toml"), []byte(tasksToml), 0644)

	reporter := NewReporter(srv.URL, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := runTask(ctx, dir, "hello", reporter)
	if err != nil {
		t.Fatalf("runTask failed: %v", err)
	}

	// Give background flushes time to arrive.
	time.Sleep(100 * time.Millisecond)

	// Check that we got output for the "hello" task stream.
	outputPath := "/api/runs/1/jobs/hello/output/hello"
	got := handler.getOutput(outputPath)
	if !strings.Contains(got, "hello from task") {
		t.Errorf("expected output to contain 'hello from task', got paths=%v, hello output=%q", handler.outputPaths(), got)
	}

	// Check that start and finish were called.
	reqs := handler.getRequests()
	var startFound, finishFound bool
	for _, r := range reqs {
		if strings.HasSuffix(r.Path, "/start") {
			startFound = true
		}
		if strings.HasSuffix(r.Path, "/done") {
			finishFound = true
			if r.Body["status"] != "success" {
				t.Errorf("expected success status, got %v", r.Body["status"])
			}
		}
	}
	if !startFound {
		t.Error("start job request not found")
	}
	if !finishFound {
		t.Error("finish job request not found")
	}
}

func TestRunTestsMultipleTasksGetSeparateStreams(t *testing.T) {
	handler := newRecordingHandler()
	srv := httptest.NewServer(handler)
	defer srv.Close()

	dir := t.TempDir()
	tasksToml := `
[[task]]
id = "both"
type = "short"
dependencies = ["a", "b"]
cmd = "true"

[[task]]
id = "a"
type = "short"
cmd = "echo output-from-a"

[[task]]
id = "b"
type = "short"
cmd = "echo output-from-b"
`
	os.WriteFile(filepath.Join(dir, "tasks.toml"), []byte(tasksToml), 0644)

	reporter := NewReporter(srv.URL, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := runTask(ctx, dir, "both", reporter)
	if err != nil {
		t.Fatalf("runTask failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	outA := handler.getOutput("/api/runs/1/jobs/both/output/a")
	outB := handler.getOutput("/api/runs/1/jobs/both/output/b")

	if !strings.Contains(outA, "output-from-a") {
		t.Errorf("expected task 'a' output, got %q (paths=%v)", outA, handler.outputPaths())
	}
	if !strings.Contains(outB, "output-from-b") {
		t.Errorf("expected task 'b' output, got %q (paths=%v)", outB, handler.outputPaths())
	}
}
