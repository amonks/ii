package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	handler.StartPendingBuild("")
	runs, _ := m.RecentRuns(10)
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}

	// Set a pending trigger.
	if err := m.SetPendingTrigger("pending-sha"); err != nil {
		t.Fatal(err)
	}

	handler.StartPendingBuild("")

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

func TestStartPendingBuildWaitsForPreviousMachine(t *testing.T) {
	m := testModel(t)

	// Track which Fly API endpoints are called.
	var waitCalled bool
	var createCalled bool
	mockFly := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/wait") {
			waitCalled = true
			// Verify it's waiting for the right machine and "destroyed" state.
			if !strings.Contains(r.URL.Path, "prev-machine-id") {
				t.Errorf("expected wait for prev-machine-id, got %s", r.URL.Path)
			}
			if r.URL.Query().Get("state") != "destroyed" {
				t.Errorf("expected wait for destroyed state, got %s", r.URL.Query().Get("state"))
			}
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/machines") {
			createCalled = true
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "new-machine-456",
			"state": "created",
		})
	}))
	t.Cleanup(mockFly.Close)

	mockRegistry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tags": []string{"deployment-01AAA"},
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
			FallbackImage: "test-image",
			Region:        "ord",
		},
	}

	if err := m.SetPendingTrigger("pending-sha"); err != nil {
		t.Fatal(err)
	}

	handler.StartPendingBuild("prev-machine-id")

	if !waitCalled {
		t.Error("expected WaitForState to be called for previous machine")
	}
	if !createCalled {
		t.Error("expected CreateMachine to be called after waiting")
	}
}

func TestTriggerDuringRestarting(t *testing.T) {
	m, handler := testTriggerHandler(t)

	// Finish all running runs, then create one in restarting state.
	runs, _ := m.RecentRuns(10)
	for _, r := range runs {
		if r.Status == "running" {
			m.FinishRun(r.ID, "success", "")
		}
	}

	run, _ := m.CreateRun("restarting-sha", "base1", "webhook")
	m.UpdateRunPhase(run.ID, "post-orchestrator", "restarting")

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
}

func TestTriggerFailsStaleRestartingRun(t *testing.T) {
	m, handler := testTriggerHandler(t)

	// Finish all running runs.
	runs, _ := m.RecentRuns(10)
	for _, r := range runs {
		if r.Status == "running" {
			m.FinishRun(r.ID, "success", "")
		}
	}

	// Create a run stuck in "restarting" with a very old started_at.
	run, _ := m.CreateRun("stale-sha", "base1", "webhook")
	m.UpdateRunPhase(run.ID, "post-orchestrator", "restarting")
	// Backdate started_at to 30 minutes ago.
	m.db.Model(&Run{}).Where("id = ?", run.ID).Update("started_at",
		time.Now().Add(-30*time.Minute).UTC().Format(time.RFC3339))

	body := `{"sha":"new-sha"}`
	req := httptest.NewRequest(http.MethodPost, "/trigger", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	// The stale run should be failed.
	stale, _, _ := m.RunWithJobs(run.ID)
	if stale.Status != "failed" {
		t.Errorf("expected stale run to be failed, got %s", stale.Status)
	}

	// A new run should have been created for the incoming SHA.
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["head_sha"] != "new-sha" {
		t.Errorf("expected new run with head_sha new-sha, got %v", resp["head_sha"])
	}
}

func TestBuildNow(t *testing.T) {
	m, handler := testTriggerHandler(t)

	t.Run("creates manual run and redirects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/build", nil)
		w := httptest.NewRecorder()
		handler.BuildNow(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("expected 303, got %d: %s", w.Code, w.Body.String())
		}

		runs, err := m.RecentRuns(10)
		if err != nil {
			t.Fatal(err)
		}
		var found bool
		for _, r := range runs {
			if r.HeadSHA == "main" && r.Trigger == "manual" {
				found = true
				// Verify redirect points to this run.
				loc := w.Header().Get("Location")
				if !strings.HasSuffix(loc, fmt.Sprintf("runs/%d", r.ID)) {
					t.Errorf("expected redirect to runs/%d, got %s", r.ID, loc)
				}
			}
		}
		if !found {
			t.Error("expected a run with HeadSHA=main and Trigger=manual")
		}
	})

	t.Run("rejects when build already running", func(t *testing.T) {
		// A run is still running from previous sub-test.
		req := httptest.NewRequest(http.MethodPost, "/build", nil)
		w := httptest.NewRecorder()
		handler.BuildNow(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestCancelRun(t *testing.T) {
	m := testModel(t)

	var stopCalled bool
	mockFly := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/stop") {
			stopCalled = true
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "mock-machine",
			"state": "stopped",
		})
	}))
	t.Cleanup(mockFly.Close)

	flyClient := flyapi.NewClient("test-token", "monks-ci-builder")
	flyClient.BaseURL = mockFly.URL

	handler := cancelRun(m, flyClient)

	t.Run("cancels a running run", func(t *testing.T) {
		run, _ := m.CreateRun("cancel-sha", "base1", "webhook")
		machineID := "mock-machine-id"
		m.SetMachineID(run.ID, machineID)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/runs/%d/cancel", run.ID), nil)
		req.SetPathValue("id", fmt.Sprintf("%d", run.ID))
		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("expected 303, got %d: %s", w.Code, w.Body.String())
		}

		// Verify run is cancelled.
		updated, _, _ := m.RunWithJobs(run.ID)
		if updated.Status != "cancelled" {
			t.Errorf("expected status cancelled, got %s", updated.Status)
		}

		if !stopCalled {
			t.Error("expected StopMachine to be called")
		}
	})

	t.Run("rejects cancel of finished run", func(t *testing.T) {
		run, _ := m.CreateRun("done-sha", "base1", "webhook")
		m.FinishRun(run.ID, "success", "")

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/runs/%d/cancel", run.ID), nil)
		req.SetPathValue("id", fmt.Sprintf("%d", run.ID))
		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("returns 404 for nonexistent run", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/runs/99999/cancel", nil)
		req.SetPathValue("id", "99999")
		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestSupersedeWaitsForOldBuilder(t *testing.T) {
	m := testModel(t)

	var calls []string
	mockFly := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/stop"):
			calls = append(calls, "stop")
		case strings.Contains(r.URL.Path, "/wait"):
			calls = append(calls, "wait")
			if r.URL.Query().Get("state") != "destroyed" {
				t.Errorf("expected wait for destroyed state, got %s", r.URL.Query().Get("state"))
			}
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/machines"):
			calls = append(calls, "create")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "new-machine",
			"state": "created",
		})
	}))
	t.Cleanup(mockFly.Close)

	mockRegistry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"tags": []string{"deployment-01AAA"}})
	}))
	t.Cleanup(mockRegistry.Close)

	flyClient := flyapi.NewClient("test-token", "monks-ci-builder")
	flyClient.BaseURL = mockFly.URL
	flyClient.RegistryURL = mockRegistry.URL

	handler := &TriggerHandler{
		model: m,
		fly:   flyClient,
		builderConfig: BuilderConfig{
			FallbackImage: "test-image",
			Region:        "ord",
		},
	}

	// Create a run to be the "new" run that supersedeAndStart will build.
	run, err := m.CreateRun("new-sha", "base1", "webhook")
	if err != nil {
		t.Fatal(err)
	}

	handler.supersedeAndStart("old-machine-id", run)

	// Verify the call order: stop, wait, then create.
	if len(calls) != 3 {
		t.Fatalf("expected 3 fly API calls, got %d: %v", len(calls), calls)
	}
	if calls[0] != "stop" {
		t.Errorf("expected first call to be stop, got %s", calls[0])
	}
	if calls[1] != "wait" {
		t.Errorf("expected second call to be wait, got %s", calls[1])
	}
	if calls[2] != "create" {
		t.Errorf("expected third call to be create, got %s", calls[2])
	}
}

func TestSupersedeFailsRunOnTimeout(t *testing.T) {
	m := testModel(t)

	// Mock fly that always returns 408 for wait (simulating the machine never dying).
	mockFly := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/wait") {
			// Simulate a brief delay so we don't spin too fast.
			time.Sleep(50 * time.Millisecond)
			http.Error(w, "timeout", http.StatusRequestTimeout)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "machine",
			"state": "stopped",
		})
	}))
	t.Cleanup(mockFly.Close)

	flyClient := flyapi.NewClient("test-token", "monks-ci-builder")
	flyClient.BaseURL = mockFly.URL

	handler := &TriggerHandler{
		model:          m,
		fly:            flyClient,
		destroyTimeout: 500 * time.Millisecond,
		builderConfig: BuilderConfig{
			FallbackImage: "test-image",
			Region:        "ord",
		},
	}

	// supersedeAndStart should fail the run when the old builder won't die.
	run, _ := m.CreateRun("timeout-sha", "base1", "webhook")
	handler.supersedeAndStart("stuck-machine", run)

	updated, _, _ := m.RunWithJobs(run.ID)
	if updated.Status != "failed" {
		t.Errorf("expected run to be failed after timeout, got %s", updated.Status)
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
