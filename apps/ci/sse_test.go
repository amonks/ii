package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"monks.co/pkg/serve"
)

func TestSSEContentType(t *testing.T) {
	m := testModel(t)
	hub := NewOutputHub()
	outputDir := t.TempDir()

	// Use a finished run so the handler returns immediately.
	run, _ := m.CreateRun("sha1", "base1", "webhook")
	m.FinishRun(run.ID, "success", "")

	mux := serve.NewMux()
	mux.HandleFunc("GET /runs/{id}/events", sseHandler(m, outputDir, hub))

	req := httptest.NewRequest(http.MethodGet, "/runs/1/events", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}
}

func TestSSEInitialEvent(t *testing.T) {
	m := testModel(t)
	hub := NewOutputHub()
	outputDir := t.TempDir()

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	m.StartJob(run.ID, "test", "go-test", filepath.Join(outputDir, "1", "go-test"))

	// Create a stream file.
	streamDir := filepath.Join(outputDir, "1", "go-test")
	os.MkdirAll(streamDir, 0755)
	os.WriteFile(filepath.Join(streamDir, "stdout.log"), []byte("hello\n"), 0644)

	// For a finished run the handler returns immediately after the initial event.
	m.FinishRun(run.ID, "success", "")

	mux := serve.NewMux()
	mux.HandleFunc("GET /runs/{id}/events", sseHandler(m, outputDir, hub))

	req := httptest.NewRequest(http.MethodGet, "/runs/1/events", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.HasPrefix(body, "data: ") {
		t.Fatalf("expected SSE data line, got %q", body)
	}

	jsonStr := strings.TrimPrefix(strings.TrimSpace(body), "data: ")
	var state runStateEvent
	if err := json.Unmarshal([]byte(jsonStr), &state); err != nil {
		t.Fatalf("parsing SSE JSON: %v", err)
	}

	if state.Run.ID != 1 {
		t.Errorf("expected run ID 1, got %d", state.Run.ID)
	}
	if state.Run.Status != "success" {
		t.Errorf("expected status success, got %s", state.Run.Status)
	}
	if len(state.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(state.Jobs))
	}
	if state.Jobs[0].Name != "go-test" {
		t.Errorf("expected job name go-test, got %s", state.Jobs[0].Name)
	}
	if streams, ok := state.Streams["go-test"]; !ok || len(streams) != 1 || streams[0].Name != "stdout" {
		t.Errorf("expected streams [{stdout ...}] for go-test, got %v", state.Streams["go-test"])
	}
}

func TestSSEPublishReachesSubscribers(t *testing.T) {
	m := testModel(t)
	hub := NewOutputHub()
	outputDir := t.TempDir()

	run, _ := m.CreateRun("sha1", "base1", "webhook")

	// Subscribe directly to the hub to verify publish works.
	key := sseRunEventsKey(run.ID)
	ch, unsub := hub.Subscribe(key)
	defer unsub()

	// Start a job and publish state.
	m.StartJob(run.ID, "test", "go-test", filepath.Join(outputDir, "1", "go-test"))
	api := &apiHandler{model: m, outputDir: outputDir, hub: hub}
	api.publishRunState(run.ID)

	// Verify we received the event on the hub.
	select {
	case data := <-ch:
		var state runStateEvent
		if err := json.Unmarshal(data, &state); err != nil {
			t.Fatalf("parsing published event: %v", err)
		}
		if len(state.Jobs) != 1 {
			t.Fatalf("expected 1 job in published event, got %d", len(state.Jobs))
		}
		if state.Jobs[0].Name != "go-test" {
			t.Errorf("expected job name go-test, got %s", state.Jobs[0].Name)
		}
	default:
		t.Fatal("expected data on hub channel after publishRunState")
	}
}

func TestSSECloseRunEvents(t *testing.T) {
	m := testModel(t)
	hub := NewOutputHub()
	outputDir := t.TempDir()

	run, _ := m.CreateRun("sha1", "base1", "webhook")

	key := sseRunEventsKey(run.ID)
	ch, _ := hub.Subscribe(key)

	api := &apiHandler{model: m, outputDir: outputDir, hub: hub}
	api.closeRunEvents(run.ID)

	// Channel should be closed.
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after closeRunEvents")
	}
}

func TestBuildRunState(t *testing.T) {
	m := testModel(t)
	outputDir := t.TempDir()

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	job, _ := m.StartJob(run.ID, "test", "go-test", filepath.Join(outputDir, "1", "go-test"))

	// Create DB streams.
	m.StartStream(job.ID, "stdout")
	m.StartStream(job.ID, "stderr")

	// Create stream files.
	streamDir := filepath.Join(outputDir, "1", "go-test")
	os.MkdirAll(streamDir, 0755)
	os.WriteFile(filepath.Join(streamDir, "stdout.log"), []byte("output"), 0644)
	os.WriteFile(filepath.Join(streamDir, "stderr.log"), []byte("errors"), 0644)

	state, err := buildRunState(m, outputDir, run.ID)
	if err != nil {
		t.Fatal(err)
	}

	if state.Run.ID != run.ID {
		t.Errorf("expected run ID %d, got %d", run.ID, state.Run.ID)
	}
	if state.Run.Status != "running" {
		t.Errorf("expected status running, got %s", state.Run.Status)
	}
	if len(state.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(state.Jobs))
	}
	streams := state.Streams["go-test"]
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}
}

func TestBuildRunStateFallsBackToFilesystem(t *testing.T) {
	m := testModel(t)
	outputDir := t.TempDir()

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	m.StartJob(run.ID, "test", "go-test", filepath.Join(outputDir, "1", "go-test"))

	// No DB streams — just files on disk (legacy behavior).
	streamDir := filepath.Join(outputDir, "1", "go-test")
	os.MkdirAll(streamDir, 0755)
	os.WriteFile(filepath.Join(streamDir, "stdout.log"), []byte("output"), 0644)

	state, err := buildRunState(m, outputDir, run.ID)
	if err != nil {
		t.Fatal(err)
	}

	streams := state.Streams["go-test"]
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream from filesystem fallback, got %d", len(streams))
	}
	if streams[0].Name != "stdout" {
		t.Errorf("expected stream name stdout, got %s", streams[0].Name)
	}
}
