package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"monks.co/pkg/serve"
)

func setupAPI(t *testing.T) (*Model, *serve.Mux) {
	t.Helper()
	m := testModel(t)
	mux := serve.NewMux()
	RegisterAPI(mux, m, nil)
	return m, mux
}

func TestAPIStartJob(t *testing.T) {
	m, mux := setupAPI(t)

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
}

func TestAPIFinishJob(t *testing.T) {
	m, mux := setupAPI(t)

	run, _ := m.CreateRun("sha1", "base1", "webhook")
	m.StartJob(run.ID, "test", "go-test")

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
	m, mux := setupAPI(t)
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

	var smsMessage string
	RegisterAPI(mux, m, func(msg string) {
		smsMessage = msg
	})

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

func TestAPIGetBaseSHA(t *testing.T) {
	m, mux := setupAPI(t)
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

func TestAPIRecordDeployment(t *testing.T) {
	m, mux := setupAPI(t)
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
