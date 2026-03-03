package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type recordingHandler struct {
	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)

	h.requests = append(h.requests, recordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Body:   body,
	})

	// Return job_id for start requests.
	if r.Method == http.MethodPut && len(r.URL.Path) > 0 {
		json.NewEncoder(w).Encode(map[string]any{"job_id": 1})
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

func TestReporterIntegration(t *testing.T) {
	handler := &recordingHandler{}
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
