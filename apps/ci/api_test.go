package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"monks.co/pkg/serve"
)

func setupAPI(t *testing.T) (*Model, *serve.Mux, string) {
	t.Helper()
	m := testModel(t)
	mux := serve.NewMux()
	outputDir := filepath.Join(t.TempDir(), "output")
	RegisterAPI(mux, m, outputDir, nil, NewOutputHub())
	return m, mux, outputDir
}

func TestAPIStartJob(t *testing.T) {
	m, mux, outputDir := setupAPI(t)

	run, err := m.CreateRun("sha1", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	body := `{"kind":"test"}`
	req := httptest.NewRequest(http.MethodPut, "/api/runs/1/jobs/go-test/start", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["job_id"] == nil {
		t.Error("expected job_id in response")
	}

	_, jobs, err := m.RunWithJobs(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "go-test" {
		t.Errorf("expected name go-test, got %s", jobs[0].Name)
	}
	if jobs[0].Status != "in_progress" {
		t.Errorf("expected status in_progress, got %s", jobs[0].Status)
	}
	expectedPath := filepath.Join(outputDir, "1", "go-test")
	if jobs[0].OutputPath == nil || *jobs[0].OutputPath != expectedPath {
		t.Errorf("expected output_path %s, got %v", expectedPath, jobs[0].OutputPath)
	}
}

func TestAPIFinishJob(t *testing.T) {
	m, mux, _ := setupAPI(t)

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	m.StartJob(run.ID, "test", "go-test", "")

	body := `{"status":"success","duration_ms":1500}`
	req := httptest.NewRequest(http.MethodPut, "/api/runs/1/jobs/go-test/done", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	_, jobs, _ := m.RunWithJobs(run.ID)
	if jobs[0].Status != "success" {
		t.Errorf("expected success, got %s", jobs[0].Status)
	}
}

func TestAPIFinishRun(t *testing.T) {
	m, mux, _ := setupAPI(t)
	m.CreateRun("sha1", "base1", "webhook")

	body := `{"status":"success"}`
	req := httptest.NewRequest(http.MethodPut, "/api/runs/1/done", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	runs, _ := m.RecentRuns(1)
	if runs[0].Status != "success" {
		t.Errorf("expected success, got %s", runs[0].Status)
	}
}

func TestAPIFinishRunSMSOnFailure(t *testing.T) {
	m := testModel(t)
	mux := serve.NewMux()
	outputDir := filepath.Join(t.TempDir(), "output")

	var smsMessage string
	RegisterAPI(mux, m, outputDir, func(msg string) {
		smsMessage = msg
	}, NewOutputHub())

	m.CreateRun("sha1", "base1", "webhook")

	body := `{"status":"failed","error":"tests failed"}`
	req := httptest.NewRequest(http.MethodPut, "/api/runs/1/done", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if smsMessage == "" {
		t.Error("expected SMS to be sent on failure")
	}
	if !strings.Contains(smsMessage, "failed") {
		t.Errorf("expected SMS to contain 'failed', got %s", smsMessage)
	}
}

func TestAPIAppendOutput(t *testing.T) {
	m, mux, outputDir := setupAPI(t)

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	m.StartJob(run.ID, "test", "go-test", filepath.Join(outputDir, "1", "go-test"))

	// First chunk.
	req := httptest.NewRequest(http.MethodPost, "/api/runs/1/jobs/go-test/output/default", strings.NewReader("line 1\n"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Second chunk appends.
	req = httptest.NewRequest(http.MethodPost, "/api/runs/1/jobs/go-test/output/default", strings.NewReader("line 2\n"))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify file contents.
	logPath := filepath.Join(outputDir, "1", "go-test", "default.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	expected := "line 1\nline 2\n"
	if string(data) != expected {
		t.Errorf("expected %q, got %q", expected, string(data))
	}
}

func TestAPIAppendOutputMultipleStreams(t *testing.T) {
	m, mux, outputDir := setupAPI(t)

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	m.StartJob(run.ID, "task", "test", filepath.Join(outputDir, "1", "test"))

	// Write to two different streams.
	req := httptest.NewRequest(http.MethodPost, "/api/runs/1/jobs/test/output/go-test", strings.NewReader("=== RUN TestFoo\n"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/runs/1/jobs/test/output/staticcheck", strings.NewReader("checking monks.co/...\n"))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	// Verify both files exist with correct content.
	data1, err := os.ReadFile(filepath.Join(outputDir, "1", "test", "go-test.log"))
	if err != nil {
		t.Fatalf("reading go-test output: %v", err)
	}
	if string(data1) != "=== RUN TestFoo\n" {
		t.Errorf("unexpected go-test content: %q", string(data1))
	}

	data2, err := os.ReadFile(filepath.Join(outputDir, "1", "test", "staticcheck.log"))
	if err != nil {
		t.Fatalf("reading staticcheck output: %v", err)
	}
	if string(data2) != "checking monks.co/...\n" {
		t.Errorf("unexpected staticcheck content: %q", string(data2))
	}
}

func TestMarkRunDead(t *testing.T) {
	m, mux, _ := setupAPI(t)
	m.CreateRun("sha1", "base1", "webhook")

	// Verify it's running.
	runs, _ := m.RecentRuns(1)
	if runs[0].Status != "running" {
		t.Fatalf("expected running, got %s", runs[0].Status)
	}

	req := httptest.NewRequest(http.MethodPost, "/runs/1/mark-dead", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should redirect back to the run page.
	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the run is now dead.
	run, _, _ := m.RunWithJobs(1)
	if run.Status != "dead" {
		t.Errorf("expected dead, got %s", run.Status)
	}
	if run.FinishedAt == nil {
		t.Error("expected finished_at to be set")
	}
	if run.Error == nil || *run.Error != "manually marked as dead" {
		t.Errorf("expected error message, got %v", run.Error)
	}
}

func TestMarkRunDeadOnlyAffectsRunning(t *testing.T) {
	m, mux, _ := setupAPI(t)
	run, _ := m.CreateRun("sha1", "base1", "webhook")
	m.FinishRun(run.ID, "success", "")

	req := httptest.NewRequest(http.MethodPost, "/runs/1/mark-dead", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should return bad request for non-running runs.
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	// Status should still be success.
	fetched, _, _ := m.RunWithJobs(1)
	if fetched.Status != "success" {
		t.Errorf("expected success, got %s", fetched.Status)
	}
}

func TestAPIGetBaseSHA(t *testing.T) {
	m, mux, _ := setupAPI(t)
	m.CreateRun("sha1", "base1", "webhook")

	req := httptest.NewRequest(http.MethodGet, "/api/runs/1/base-sha", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["base_sha"] != "base1" {
		t.Errorf("expected base1, got %s", resp["base_sha"])
	}
}

func TestAPIAppendOutputPublishesToHub(t *testing.T) {
	m := testModel(t)
	mux := serve.NewMux()
	outputDir := filepath.Join(t.TempDir(), "output")
	hub := NewOutputHub()
	RegisterAPI(mux, m, outputDir, nil, hub)

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	m.StartJob(run.ID, "test", "go-test", filepath.Join(outputDir, "1", "go-test"))

	// Subscribe before appending.
	ch, unsub := hub.Subscribe("1/go-test/default")
	defer unsub()

	req := httptest.NewRequest(http.MethodPost, "/api/runs/1/jobs/go-test/output/default", strings.NewReader("hello\n"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	select {
	case data := <-ch:
		if string(data) != "hello\n" {
			t.Errorf("expected %q, got %q", "hello\n", string(data))
		}
	default:
		t.Error("expected data on hub channel")
	}
}

func TestAPIFinishJobClosesHub(t *testing.T) {
	m := testModel(t)
	mux := serve.NewMux()
	outputDir := filepath.Join(t.TempDir(), "output")
	hub := NewOutputHub()
	RegisterAPI(mux, m, outputDir, nil, hub)

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	m.StartJob(run.ID, "test", "go-test", filepath.Join(outputDir, "1", "go-test"))

	// Subscribe to a stream for this job.
	ch, _ := hub.Subscribe("1/go-test/default")

	body := `{"status":"success","duration_ms":100}`
	req := httptest.NewRequest(http.MethodPut, "/api/runs/1/jobs/go-test/done", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Channel should be closed.
	_, ok := <-ch
	if ok {
		t.Error("expected hub channel to be closed after finishJob")
	}
}

func TestServeStreamReturnsExistingContent(t *testing.T) {
	outputDir := t.TempDir()
	hub := NewOutputHub()

	// Create a stream file.
	dir := filepath.Join(outputDir, "1", "go-test")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "stdout.log"), []byte("line one\nline two\n"), 0644)

	mux := serve.NewMux()
	mux.HandleFunc("GET /output/{runID}/{jobName}/{stream}", serveStream(outputDir, hub))

	req := httptest.NewRequest(http.MethodGet, "/output/1/go-test/stdout", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "line one\nline two\n" {
		t.Errorf("expected file content, got %q", w.Body.String())
	}
}

func TestServeStreamEmptyFile(t *testing.T) {
	outputDir := t.TempDir()
	hub := NewOutputHub()

	mux := serve.NewMux()
	mux.HandleFunc("GET /output/{runID}/{jobName}/{stream}", serveStream(outputDir, hub))

	// No file exists at all.
	req := httptest.NewRequest(http.MethodGet, "/output/1/go-test/stdout", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReadLastLine(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"single line", "hello", "hello"},
		{"with trailing newline", "hello\n", "hello"},
		{"multiple lines", "line1\nline2\nline3\n", "line3"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name+".log")
			os.WriteFile(path, []byte(tt.content), 0644)
			got := readLastLine(path)
			if got != tt.want {
				t.Errorf("readLastLine(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestFinishRunEmitsTaskEvent(t *testing.T) {
	// Capture slog output to verify the task event.
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	origLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(origLogger)

	m, mux, _ := setupAPI(t)

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	job, _ := m.StartJob(run.ID, "test", "go-test", "")
	m.FinishJob(job.ID, "success", 1500, "", "")

	body := `{"status":"success"}`
	req := httptest.NewRequest(http.MethodPut, "/api/runs/1/done", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse the slog output to find the task event.
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var taskEvent map[string]any
	for _, line := range lines {
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev["msg"] == "task" {
			taskEvent = ev
			break
		}
	}

	if taskEvent == nil {
		t.Fatal("expected a 'task' log event to be emitted")
	}

	if taskEvent["task.name"] != "ci-run" {
		t.Errorf("task.name = %v, want ci-run", taskEvent["task.name"])
	}
	if taskEvent["task.status"] != "success" {
		t.Errorf("task.status = %v, want success", taskEvent["task.status"])
	}
	if taskEvent["run.head_sha"] != "sha1" {
		t.Errorf("run.head_sha = %v, want sha1", taskEvent["run.head_sha"])
	}
	if taskEvent["run.trigger"] != "webhook" {
		t.Errorf("run.trigger = %v, want webhook", taskEvent["run.trigger"])
	}
	// run.id should be present as a number.
	if taskEvent["run.id"] == nil {
		t.Error("expected run.id in task event")
	}
	// Job status should be included.
	if taskEvent["job.go-test.status"] != "success" {
		t.Errorf("job.go-test.status = %v, want success", taskEvent["job.go-test.status"])
	}
}

func TestFinishRunEmitsTaskEventWithDeployData(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	origLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(origLogger)

	m, mux, _ := setupAPI(t)

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	job, _ := m.StartJob(run.ID, "deploy", "deploy", "")
	m.FinishJob(job.ID, "success", 5000, "", "")

	// Deploy data is now sent as part of the finishRun request.
	body := `{"status":"success","deploys":[{"app":"dogs","image_ref":"registry.fly.io/monks-dogs:sha1","compile_ms":1000,"push_ms":2000,"deploy_ms":1500,"image_bytes":50000}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/runs/1/done", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var taskEvent map[string]any
	for _, line := range lines {
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev["msg"] == "task" {
			taskEvent = ev
			break
		}
	}

	if taskEvent == nil {
		t.Fatal("expected a 'task' log event")
	}

	if taskEvent["deploy.dogs.image_ref"] != "registry.fly.io/monks-dogs:sha1" {
		t.Errorf("deploy.dogs.image_ref = %v", taskEvent["deploy.dogs.image_ref"])
	}
	// JSON numbers are float64.
	if v, ok := taskEvent["deploy.dogs.compile_ms"].(float64); !ok || v != 1000 {
		t.Errorf("deploy.dogs.compile_ms = %v", taskEvent["deploy.dogs.compile_ms"])
	}
}

func TestAPIStartAndFinishStream(t *testing.T) {
	m, mux, _ := setupAPI(t)

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	m.StartJob(run.ID, "deploy", "deploy", "")

	// Start a stream.
	req := httptest.NewRequest(http.MethodPut, "/api/runs/1/jobs/deploy/streams/dogs/start", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["stream_id"] == nil {
		t.Error("expected stream_id in response")
	}

	// Finish the stream.
	body := `{"status":"success","duration_ms":2000}`
	req = httptest.NewRequest(http.MethodPut, "/api/runs/1/jobs/deploy/streams/dogs/done", strings.NewReader(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify stream in DB.
	_, jobs, _ := m.RunWithJobs(run.ID)
	streams, err := m.StreamsForJob(jobs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(streams))
	}
	if streams[0].Status != "success" {
		t.Errorf("expected success, got %s", streams[0].Status)
	}
	if streams[0].DurationMs == nil || *streams[0].DurationMs != 2000 {
		t.Error("expected duration_ms 2000")
	}
}

func TestAPIRecordDeployment(t *testing.T) {
	m, mux, _ := setupAPI(t)
	m.CreateRun("sha1", "base1", "webhook")

	body := `{"app":"dogs","commit_sha":"sha1","image_ref":"registry.fly.io/monks-dogs:sha1","binary_bytes":1024}`
	req := httptest.NewRequest(http.MethodPost, "/api/runs/1/deployments", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	current, err := m.CurrentDeployments()
	if err != nil {
		t.Fatal(err)
	}
	if len(current) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(current))
	}
	if current[0].App != "dogs" {
		t.Errorf("expected app dogs, got %s", current[0].App)
	}
}
