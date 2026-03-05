package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"monks.co/pkg/flyapi"
)

func testTriggerHandler(t *testing.T) (*Model, *TriggerHandler) {
	t.Helper()
	m := testModel(t)

	// Mock fly API server that handles machine creation.
	mockFly := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "mock-machine-123",
			"state": "created",
		})
	}))
	t.Cleanup(mockFly.Close)

	// Mock registry that returns tags.
	mockRegistry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tags": []string{"deployment-01AAA", "deployment-01BBB"},
		})
	}))
	t.Cleanup(mockRegistry.Close)

	flyClient := flyapi.NewClient("test-token", "monks-ci-builder")
	flyClient.BaseURL = mockFly.URL
	flyClient.RegistryURL = mockRegistry.URL

	handler := &TriggerHandler{
		model: m,
		fly:   flyClient,
		builderConfig: BuilderConfig{
			FallbackImage: "test-image-fallback",
			Region:        "ord",
		},
	}
	return m, handler
}

func TestTriggerHandler(t *testing.T) {
	m, handler := testTriggerHandler(t)

	t.Run("creates run on valid request", func(t *testing.T) {
		body := `{"sha":"abc123"}`
		req := httptest.NewRequest(http.MethodPost, "/trigger", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["head_sha"] != "abc123" {
			t.Errorf("expected head_sha abc123, got %v", resp["head_sha"])
		}

		// Verify run was created.
		runs, err := m.RecentRuns(10)
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) != 1 {
			t.Fatalf("expected 1 run, got %d", len(runs))
		}
		if runs[0].HeadSHA != "abc123" {
			t.Errorf("expected head sha abc123, got %s", runs[0].HeadSHA)
		}
	})

	t.Run("rejects GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/trigger", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("rejects empty sha", func(t *testing.T) {
		body := `{"sha":""}`
		req := httptest.NewRequest(http.MethodPost, "/trigger", strings.NewReader(body))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("supersedes pre-deploy run", func(t *testing.T) {
		// First run is still running from previous test (no jobs started = pre-deploy).
		body := `{"sha":"def456"}`
		req := httptest.NewRequest(http.MethodPost, "/trigger", strings.NewReader(body))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["head_sha"] != "def456" {
			t.Errorf("expected head_sha def456, got %v", resp["head_sha"])
		}

		// The old run should eventually be superseded (done in a goroutine).
		// The new run should exist.
		runs, _ := m.RecentRuns(10)
		var hasNewRun bool
		for _, r := range runs {
			if r.HeadSHA == "def456" && r.Status == "running" {
				hasNewRun = true
			}
		}
		if !hasNewRun {
			t.Error("expected a new running run for def456")
		}
	})

	t.Run("queues during deploy", func(t *testing.T) {
		// Finish all running runs, then create one in deploy phase.
		runs, _ := m.RecentRuns(10)
		for _, r := range runs {
			if r.Status == "running" {
				m.FinishRun(r.ID, "success", "")
			}
		}

		run, _ := m.CreateRun("deploy-sha", "base1", "webhook")
		m.StartJob(run.ID, "deploy", "deploy", "")

		body := `{"sha":"queued-sha"}`
		req := httptest.NewRequest(http.MethodPost, "/trigger", strings.NewReader(body))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["status"] != "queued" {
			t.Errorf("expected status queued, got %v", resp["status"])
		}

		// Verify pending trigger was set.
		sha, ok, err := m.PopPendingTrigger()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("expected pending trigger to be set")
		}
		if sha != "queued-sha" {
			t.Errorf("expected queued-sha, got %s", sha)
		}
	})
}

func TestStartPendingBuild(t *testing.T) {
	m, handler := testTriggerHandler(t)

	// No pending trigger — should be a no-op.
	handler.StartPendingBuild()
	runs, _ := m.RecentRuns(10)
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}

	// Set a pending trigger.
	if err := m.SetPendingTrigger("pending-sha"); err != nil {
		t.Fatal(err)
	}

	handler.StartPendingBuild()

	runs, _ = m.RecentRuns(10)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].HeadSHA != "pending-sha" {
		t.Errorf("expected head_sha pending-sha, got %s", runs[0].HeadSHA)
	}
	if runs[0].Trigger != "pending" {
		t.Errorf("expected trigger pending, got %s", runs[0].Trigger)
	}

	// Pending trigger should be consumed.
	_, ok, _ := m.PopPendingTrigger()
	if ok {
		t.Error("expected pending trigger to be consumed")
	}
}

func TestTriggerHandlerNoFlyClient(t *testing.T) {
	m := testModel(t)
	handler := &TriggerHandler{
		model: m,
		// No fly client.
	}

	body := `{"sha":"abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/trigger", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when fly client missing, got %d", w.Code)
	}
}
