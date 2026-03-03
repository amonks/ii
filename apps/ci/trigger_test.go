package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTriggerHandler(t *testing.T) {
	m := testModel(t)

	handler := &TriggerHandler{
		model: m,
		// No fly client — skips machine creation.
	}

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

	t.Run("skips when run already in progress", func(t *testing.T) {
		// First run is still running from previous test.
		body := `{"sha":"def456"}`
		req := httptest.NewRequest(http.MethodPost, "/trigger", strings.NewReader(body))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d", w.Code)
		}
	})

	t.Run("allows new run after previous finishes", func(t *testing.T) {
		// Finish the running run.
		runs, _ := m.RecentRuns(10)
		for _, r := range runs {
			if r.Status == "running" {
				m.FinishRun(r.ID, "success")
			}
		}

		body := `{"sha":"ghi789"}`
		req := httptest.NewRequest(http.MethodPost, "/trigger", strings.NewReader(body))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
		}

		// Base SHA should be abc123 (the last successful run's head).
		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["base_sha"] != "abc123" {
			t.Errorf("expected base_sha abc123, got %v", resp["base_sha"])
		}
	})
}
