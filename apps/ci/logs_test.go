package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchRunLogs_ParsesTaskEvent(t *testing.T) {
	runID := int64(42)

	// Simulate a logs /events response containing a task event
	// matching this run and an unrelated task event for a different run.
	eventsResp := map[string]any{
		"total": 2,
		"events": []map[string]any{
			{
				"id":        1,
				"timestamp": "2026-03-01T10:30:00Z",
				"app":       "ci",
				"data": map[string]any{
					"time":             "2026-03-01T10:30:00Z",
					"level":            "INFO",
					"msg":              "task",
					"app.name":         "ci",
					"task.name":        "ci-run",
					"task.status":      "success",
					"task.duration_ms":  60000,
					"run.id":           float64(runID),
					"run.head_sha":     "abc123",
					"run.base_sha":     "def456",
					"run.trigger":      "webhook",
					"job.test.status":  "success",
				},
			},
			{
				"id":        2,
				"timestamp": "2026-03-01T10:31:00Z",
				"app":       "ci",
				"data": map[string]any{
					"time":             "2026-03-01T10:31:00Z",
					"level":            "INFO",
					"msg":              "task",
					"app.name":         "ci",
					"task.name":        "ci-run",
					"task.status":      "failed",
					"run.id":           float64(99), // different run
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the query includes msg:task.
		q := r.URL.Query().Get("q")
		if q != "group:app,app:ci,msg:task" {
			t.Errorf("expected query group:app,app:ci,msg:task, got %q", q)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(eventsResp)
	}))
	defer srv.Close()

	run := &Run{
		ID:        runID,
		StartedAt: "2026-03-01T10:29:00Z",
	}

	events, err := fetchRunLogsFrom(srv.URL, http.DefaultClient, run)
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event matching run %d, got %d", runID, len(events))
	}

	ev := events[0]
	if ev.Data["task.name"] != "ci-run" {
		t.Errorf("task.name = %v, want ci-run", ev.Data["task.name"])
	}
	if ev.Data["task.status"] != "success" {
		t.Errorf("task.status = %v, want success", ev.Data["task.status"])
	}
	if ev.Data["run.head_sha"] != "abc123" {
		t.Errorf("run.head_sha = %v, want abc123", ev.Data["run.head_sha"])
	}
}

func TestFetchRunLogs_NoMatchingEvents(t *testing.T) {
	eventsResp := map[string]any{
		"total":  0,
		"events": []map[string]any{},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(eventsResp)
	}))
	defer srv.Close()

	run := &Run{
		ID:        1,
		StartedAt: "2026-03-01T10:29:00Z",
	}

	events, err := fetchRunLogsFrom(srv.URL, http.DefaultClient, run)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestSortedDataKeys(t *testing.T) {
	data := map[string]any{
		"time":              "2026-03-01T10:30:00Z",
		"level":             "INFO",
		"msg":               "task",
		"app.name":          "ci",
		"task.name":         "ci-run",
		"task.status":       "success",
		"task.duration_ms":  60000,
		"run.id":            42,
		"run.head_sha":      "abc123",
		"job.test.status":   "success",
		"stream.deploy.dogs.status": "success",
		"deploy.dogs.image_ref": "registry.fly.io/monks-dogs:sha1",
	}

	keys := SortedDataKeys(data)

	// Should not include time, level, msg, app.name.
	for _, k := range keys {
		switch k {
		case "time", "level", "msg", "app.name":
			t.Errorf("unexpected key %q in sorted keys", k)
		}
	}

	// Should be ordered: task.*, run.*, job.*, stream.*, deploy.*
	if len(keys) != 8 {
		t.Fatalf("expected 8 keys, got %d: %v", len(keys), keys)
	}

	// Verify group ordering: task < run < job < stream < deploy.
	groupOf := func(key string) string {
		for i, c := range key {
			if c == '.' {
				return key[:i]
			}
		}
		return key
	}

	prevGroup := ""
	for _, k := range keys {
		g := groupOf(k)
		if prevGroup != "" && g != prevGroup {
			// Group changed; verify it's in the right order.
			order := map[string]int{"task": 0, "run": 1, "job": 2, "stream": 3, "deploy": 4}
			if order[g] < order[prevGroup] {
				t.Errorf("key %q (group %q) appeared after group %q", k, g, prevGroup)
			}
		}
		prevGroup = g
	}
}
