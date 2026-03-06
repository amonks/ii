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

func requestPaths(reqs []recordedRequest) []string {
	paths := make([]string, len(reqs))
	for i, r := range reqs {
		paths[i] = r.Path
	}
	return paths
}

func TestReporterIntegration(t *testing.T) {
	handler := newRecordingHandler()
	srv := httptest.NewServer(handler)
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)

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

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := runTask(ctx, dir, "hello", "hello", reporter)
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

func TestRunTestsSubmoduleTasksEncodeStreamNames(t *testing.T) {
	handler := newRecordingHandler()
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Create a root dir with a task that depends on a sub-module task.
	dir := t.TempDir()
	rootTasksToml := `
[[task]]
id = "generate"
type = "short"
dependencies = ["sub/build"]

[[task]]
id = "local"
type = "short"
cmd = "echo local"
`
	os.WriteFile(filepath.Join(dir, "tasks.toml"), []byte(rootTasksToml), 0644)

	// Create the sub-module task.
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0755)
	subTasksToml := `
[[task]]
id = "build"
type = "short"
cmd = "echo sub-output"
`
	os.WriteFile(filepath.Join(subDir, "tasks.toml"), []byte(subTasksToml), 0644)

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := runTask(ctx, dir, "generate", "generate", reporter)
	if err != nil {
		t.Fatalf("runTask failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// The sub-module task "sub/build" should have its stream name encoded
	// as "sub~build" so it works as a single URL path segment.
	outSub := handler.getOutput("/api/runs/1/jobs/generate/output/sub~build")
	if !strings.Contains(outSub, "sub-output") {
		t.Errorf("expected sub/build output at encoded path, got %q (paths=%v)", outSub, handler.outputPaths())
	}

	// Check that stream start/finish use encoded names too.
	reqs := handler.getRequests()
	var foundStreamStart bool
	for _, r := range reqs {
		if r.Path == "/api/runs/1/jobs/generate/streams/sub~build/start" {
			foundStreamStart = true
		}
	}
	if !foundStreamStart {
		var paths []string
		for _, r := range reqs {
			paths = append(paths, r.Path)
		}
		t.Errorf("expected stream start for sub~build, got paths=%v", paths)
	}
}

func TestRunTestsRunsGenerateThenCITest(t *testing.T) {
	handler := newRecordingHandler()
	srv := httptest.NewServer(handler)
	defer srv.Close()

	dir := t.TempDir()
	tasksToml := `
[[task]]
id = "generate"
type = "short"
cmd = "echo generating"

[[task]]
id = "test"
type = "short"
cmd = "echo testing"

[[task]]
id = "check-for-diff"
type = "short"
cmd = "echo no diff"

[[task]]
id = "ci-test"
type = "short"
dependencies = ["test", "check-for-diff"]
`
	os.WriteFile(filepath.Join(dir, "tasks.toml"), []byte(tasksToml), 0644)

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := RunTests(ctx, dir, reporter, "")
	if err != nil {
		t.Fatalf("RunTests failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify that both "generate" and "ci-test" jobs were started.
	reqs := handler.getRequests()
	var generateStart, ciTestStart bool
	for _, r := range reqs {
		if r.Path == "/api/runs/1/jobs/generate/start" {
			generateStart = true
		}
		if r.Path == "/api/runs/1/jobs/ci-test/start" {
			ciTestStart = true
		}
	}
	if !generateStart {
		t.Error("expected generate job to be started")
	}
	if !ciTestStart {
		t.Error("expected ci-test job to be started (not plain 'test')")
	}

	// Verify check-for-diff ran as a stream within ci-test.
	diffOutput := handler.getOutput("/api/runs/1/jobs/ci-test/output/check-for-diff")
	if !strings.Contains(diffOutput, "no diff") {
		t.Errorf("expected check-for-diff output, got %q (paths=%v)", diffOutput, handler.outputPaths())
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

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := runTask(ctx, dir, "both", "both", reporter)
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

func TestRunTestsJobNameSuffix(t *testing.T) {
	handler := newRecordingHandler()
	srv := httptest.NewServer(handler)
	defer srv.Close()

	dir := t.TempDir()
	tasksToml := `
[[task]]
id = "generate"
type = "short"
cmd = "echo generating"

[[task]]
id = "ci-test"
type = "short"
cmd = "echo testing"
`
	os.WriteFile(filepath.Join(dir, "tasks.toml"), []byte(tasksToml), 0644)

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := RunTests(ctx, dir, reporter, "-post-orchestrator")
	if err != nil {
		t.Fatalf("RunTests failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify that suffixed job names were used.
	reqs := handler.getRequests()
	var generateStart, ciTestStart bool
	for _, r := range reqs {
		if r.Path == "/api/runs/1/jobs/generate-post-orchestrator/start" {
			generateStart = true
		}
		if r.Path == "/api/runs/1/jobs/ci-test-post-orchestrator/start" {
			ciTestStart = true
		}
	}
	if !generateStart {
		t.Errorf("expected generate-post-orchestrator job to be started; got paths: %v", requestPaths(reqs))
	}
	if !ciTestStart {
		t.Errorf("expected ci-test-post-orchestrator job to be started; got paths: %v", requestPaths(reqs))
	}
}
