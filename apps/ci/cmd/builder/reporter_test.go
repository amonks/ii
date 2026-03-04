package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReporterStartJob(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 42, http.DefaultClient)
	if err := reporter.StartJob("go-test", "test"); err != nil {
		t.Fatal(err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", gotMethod)
	}
	if gotPath != "/api/runs/42/jobs/go-test/start" {
		t.Errorf("expected /api/runs/42/jobs/go-test/start, got %s", gotPath)
	}
	if gotBody["kind"] != "test" {
		t.Errorf("expected kind test, got %s", gotBody["kind"])
	}
}

func TestReporterFinishJob(t *testing.T) {
	var gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		bs, _ := io.ReadAll(r.Body)
		json.Unmarshal(bs, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	err := reporter.FinishJob("go-test", FinishJobResult{
		Status:     "success",
		DurationMs: 1500,
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/api/runs/1/jobs/go-test/done" {
		t.Errorf("expected /api/runs/1/jobs/go-test/done, got %s", gotPath)
	}
	if gotBody["status"] != "success" {
		t.Errorf("expected status success, got %v", gotBody["status"])
	}
}

func TestReporterStartStream(t *testing.T) {
	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	if err := reporter.StartStream("deploy", "dogs"); err != nil {
		t.Fatal(err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", gotMethod)
	}
	if gotPath != "/api/runs/1/jobs/deploy/streams/dogs/start" {
		t.Errorf("expected /api/runs/1/jobs/deploy/streams/dogs/start, got %s", gotPath)
	}
}

func TestReporterFinishStream(t *testing.T) {
	var gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		bs, _ := io.ReadAll(r.Body)
		json.Unmarshal(bs, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	err := reporter.FinishStream("deploy", "dogs", FinishStreamResult{
		Status:     "success",
		DurationMs: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/api/runs/1/jobs/deploy/streams/dogs/done" {
		t.Errorf("expected /api/runs/1/jobs/deploy/streams/dogs/done, got %s", gotPath)
	}
	if gotBody["status"] != "success" {
		t.Errorf("expected status success, got %v", gotBody["status"])
	}
}

func TestReporterFinishRun(t *testing.T) {
	var gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		bs, _ := io.ReadAll(r.Body)
		json.Unmarshal(bs, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 5, http.DefaultClient)
	if err := reporter.FinishRun("failed", "tests failed"); err != nil {
		t.Fatal(err)
	}

	if gotPath != "/api/runs/5/done" {
		t.Errorf("expected /api/runs/5/done, got %s", gotPath)
	}
	if gotBody["status"] != "failed" {
		t.Errorf("expected status failed, got %v", gotBody["status"])
	}
	if gotBody["error"] != "tests failed" {
		t.Errorf("expected error 'tests failed', got %v", gotBody["error"])
	}
}

func TestReporterFinishRunWithDeploys(t *testing.T) {
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bs, _ := io.ReadAll(r.Body)
		json.Unmarshal(bs, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	reporter.AddDeployResult(DeployResult{
		App:      "dogs",
		ImageRef: "registry.fly.io/monks-dogs:sha1",
	})

	if err := reporter.FinishRun("success", ""); err != nil {
		t.Fatal(err)
	}

	deploys, ok := gotBody["deploys"].([]any)
	if !ok || len(deploys) != 1 {
		t.Fatalf("expected 1 deploy in request body, got %v", gotBody["deploys"])
	}
	d := deploys[0].(map[string]any)
	if d["app"] != "dogs" {
		t.Errorf("expected deploy app dogs, got %v", d["app"])
	}
}

func TestReporterGetBaseSHA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/runs/3/base-sha" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]string{"base_sha": "abc123"})
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 3, http.DefaultClient)
	sha, err := reporter.GetBaseSHA()
	if err != nil {
		t.Fatal(err)
	}
	if sha != "abc123" {
		t.Errorf("expected abc123, got %s", sha)
	}
}

func TestReporterRecordDeployment(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		bs, _ := io.ReadAll(r.Body)
		json.Unmarshal(bs, &gotBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 7, http.DefaultClient)
	err := reporter.RecordDeployment("dogs", "sha1", "registry.fly.io/monks-dogs:sha1", 1024)
	if err != nil {
		t.Fatal(err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/runs/7/deployments" {
		t.Errorf("expected /api/runs/7/deployments, got %s", gotPath)
	}
	if gotBody["app"] != "dogs" {
		t.Errorf("expected app dogs, got %v", gotBody["app"])
	}
}

func TestReporterErrorOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 1, http.DefaultClient)
	err := reporter.StartJob("test", "test")
	if err == nil {
		t.Fatal("expected error on 400 response")
	}
}

func TestRetryDoRetriesOnConnectionError(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Force close the connection without responding.
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("server doesn't support hijacking")
			}
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := retryConfig{maxAttempts: 5, baseDelay: time.Millisecond, maxDelay: 10 * time.Millisecond}
	resp, err := retryDo(http.DefaultClient, func() (*http.Request, error) {
		return http.NewRequest("GET", srv.URL, nil)
	}, cfg)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	resp.Body.Close()
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryDoRetriesOn5xx(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := retryConfig{maxAttempts: 5, baseDelay: time.Millisecond, maxDelay: 10 * time.Millisecond}
	resp, err := retryDo(http.DefaultClient, func() (*http.Request, error) {
		return http.NewRequest("GET", srv.URL, nil)
	}, cfg)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	resp.Body.Close()
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryDoDoesNotRetry4xx(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	cfg := retryConfig{maxAttempts: 5, baseDelay: time.Millisecond, maxDelay: 10 * time.Millisecond}
	resp, err := retryDo(http.DefaultClient, func() (*http.Request, error) {
		return http.NewRequest("GET", srv.URL, nil)
	}, cfg)
	if err != nil {
		t.Fatalf("expected no error (4xx is returned, not retried), got: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestRetryDoExhaustsAttempts(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := retryConfig{maxAttempts: 3, baseDelay: time.Millisecond, maxDelay: 10 * time.Millisecond}
	_, err := retryDo(http.DefaultClient, func() (*http.Request, error) {
		return http.NewRequest("GET", srv.URL, nil)
	}, cfg)
	if err == nil {
		t.Fatal("expected error after exhausting attempts")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}
