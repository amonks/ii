package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLastNLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"empty", "", 5, "\n"},
		{"fewer than n", "a\nb\nc\n", 5, "a\nb\nc\n"},
		{"exact n", "a\nb\nc\n", 3, "a\nb\nc\n"},
		{"more than n", "a\nb\nc\nd\ne\n", 3, "c\nd\ne\n"},
		{"no trailing newline", "a\nb\nc", 2, "b\nc\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastNLines(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("lastNLines(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

func TestFindFirstFailure(t *testing.T) {
	state := &runState{
		Run: runJSON{ID: 1, Status: "failed"},
		Jobs: []jobJSON{
			{Name: "fetch", Status: "success"},
			{Name: "test", Status: "failed"},
			{Name: "deploy", Status: "failed"}, // should be skipped
		},
		Streams: map[string][]streamJSON{
			"fetch": {{Name: "go-deps", Status: "success"}},
			"test": {
				{Name: "monks.co", Status: "failed"},
				{Name: "monks.co~pkg~serve", Status: "success"},
			},
			"deploy": {{Name: "monks-proxy", Status: "failed"}},
		},
	}

	job, stream, found := findFirstFailure(state)
	if !found {
		t.Fatal("expected to find a failure")
	}
	if job != "test" {
		t.Errorf("job = %q, want %q", job, "test")
	}
	if stream != "monks.co" {
		t.Errorf("stream = %q, want %q", stream, "monks.co")
	}
}

func TestFindFirstFailure_NoFailure(t *testing.T) {
	state := &runState{
		Run:  runJSON{ID: 1, Status: "success"},
		Jobs: []jobJSON{{Name: "test", Status: "success"}},
	}
	_, _, found := findFirstFailure(state)
	if found {
		t.Error("expected no failure found")
	}
}

func TestResolveRunID_Latest(t *testing.T) {
	runs := []apiRun{{ID: 42, Status: "success", HeadSHA: "abc123", StartedAt: "2026-01-01T00:00:00Z", Trigger: "webhook"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(runs)
	}))
	defer srv.Close()

	id, err := resolveRunID(srv.URL, "latest")
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
}

func TestResolveRunID_Numeric(t *testing.T) {
	id, err := resolveRunID("http://unused", "123")
	if err != nil {
		t.Fatal(err)
	}
	if id != 123 {
		t.Errorf("id = %d, want 123", id)
	}
}

func TestResolveRunID_Invalid(t *testing.T) {
	_, err := resolveRunID("http://unused", "notanumber")
	if err == nil {
		t.Error("expected error for invalid run ID")
	}
}

func TestStatusStr(t *testing.T) {
	tests := map[string]string{
		"success":    "ok",
		"failed":     "FAIL",
		"running":    "...",
		"in_progress": "...",
		"pending":    "   ",
		"skipped":    "skip",
		"cancelled":  "cancel",
		"superseded": "super",
		"unknown":    "unknown",
	}
	for input, want := range tests {
		if got := statusStr(input); got != want {
			t.Errorf("statusStr(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []string{"success", "failed", "cancelled", "superseded"}
	for _, s := range terminal {
		if !isTerminal(s) {
			t.Errorf("isTerminal(%q) = false, want true", s)
		}
	}
	nonTerminal := []string{"running", "restarting", "pending"}
	for _, s := range nonTerminal {
		if isTerminal(s) {
			t.Errorf("isTerminal(%q) = true, want false", s)
		}
	}
}

func TestWaitForRun_ImmediateSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(runState{
			Run: runJSON{ID: 1, Status: "success"},
		})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	err := waitForRun(srv.URL, 1, 10*time.Millisecond, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "success") {
		t.Errorf("output = %q, want it to contain 'success'", buf.String())
	}
}

func TestWaitForRun_ImmediateFailure(t *testing.T) {
	errMsg := "build failed"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(runState{
			Run: runJSON{ID: 1, Status: "failed", Error: &errMsg},
		})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	err := waitForRun(srv.URL, 1, 10*time.Millisecond, &buf)
	if err == nil {
		t.Fatal("expected error for failed run")
	}
	if !strings.Contains(err.Error(), "build failed") {
		t.Errorf("error = %q, want it to contain 'build failed'", err.Error())
	}
}

func TestWaitForRun_TransitionsToSuccess(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		status := "running"
		if n >= 3 {
			status = "success"
		}
		state := runState{
			Run:  runJSON{ID: 1, Status: status},
			Jobs: []jobJSON{{Name: "test", Status: "in_progress"}},
		}
		if status == "success" {
			state.Jobs[0].Status = "success"
		}
		json.NewEncoder(w).Encode(state)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	err := waitForRun(srv.URL, 1, 10*time.Millisecond, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "running") {
		t.Errorf("output should contain 'running', got %q", output)
	}
	if !strings.Contains(output, "success") {
		t.Errorf("output should contain 'success', got %q", output)
	}
}

func TestWaitForRun_ShowsCurrentJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(runState{
			Run:  runJSON{ID: 1, Status: "success"},
			Jobs: []jobJSON{{Name: "deploy", Kind: "deploy", Status: "in_progress"}},
		})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	err := waitForRun(srv.URL, 1, 10*time.Millisecond, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// success is terminal so it prints status without job info (job won't be in_progress at that point)
	if !strings.Contains(buf.String(), "success") {
		t.Errorf("output = %q, want it to contain 'success'", buf.String())
	}
}

func TestPrintRunDetail_Success(t *testing.T) {
	state := &runState{
		Run: runJSON{ID: 1, Status: "success", HeadSHA: "abc12345dead", Trigger: "webhook", StartedAt: "2026-01-01T00:00:00Z"},
		Jobs: []jobJSON{
			{Name: "fetch", Status: "success"},
			{Name: "test", Status: "success"},
			{Name: "deploy", Status: "success"},
		},
	}

	var buf bytes.Buffer
	printRunDetail(&buf, state, "", "", nil)
	out := buf.String()

	// Should show run header
	if !strings.Contains(out, "Run 1") {
		t.Errorf("expected run header, got %q", out)
	}
	// Should NOT list any jobs (all succeeded)
	if strings.Contains(out, "fetch") || strings.Contains(out, "test") || strings.Contains(out, "deploy") {
		t.Errorf("should not list succeeded jobs, got %q", out)
	}
}

func TestPrintRunDetail_Failure(t *testing.T) {
	errMsg := "exit status 1"
	state := &runState{
		Run: runJSON{ID: 5, Status: "failed", HeadSHA: "abc12345dead", Trigger: "push", StartedAt: "2026-01-01T00:00:00Z"},
		Jobs: []jobJSON{
			{Name: "fetch", Status: "success"},
			{Name: "test", Status: "failed", Error: &errMsg},
			{Name: "deploy", Status: "failed"}, // cascade failure
		},
		Streams: map[string][]streamJSON{
			"fetch": {{Name: "go-deps", DisplayName: "go-deps", Status: "success"}},
			"test": {
				{Name: "monks.co", DisplayName: "monks.co", Status: "failed", Error: &errMsg},
				{Name: "monks.co~pkg~serve", DisplayName: "pkg/serve", Status: "success"},
			},
			"deploy": {{Name: "monks-proxy", DisplayName: "monks-proxy", Status: "failed"}},
		},
	}

	logContent := "line1\nline2\nline3\nline4\nline5\n"
	var buf bytes.Buffer
	printRunDetail(&buf, state, "test", "monks.co", func(job, stream string, n int) string {
		if job == "test" && stream == "monks.co" {
			return lastNLines(logContent, n)
		}
		return ""
	})
	out := buf.String()

	// Should show run header
	if !strings.Contains(out, "Run 5") {
		t.Errorf("expected run header, got %q", out)
	}
	// Should NOT show successful jobs
	if strings.Contains(out, "fetch") {
		t.Errorf("should not show succeeded job 'fetch', got %q", out)
	}
	// Should show the first failing job and stream
	if !strings.Contains(out, "test") {
		t.Errorf("should show first failing job 'test', got %q", out)
	}
	if !strings.Contains(out, "monks.co") {
		t.Errorf("should show first failing stream, got %q", out)
	}
	// Should show log tail
	if !strings.Contains(out, "line1") {
		t.Errorf("should show log content, got %q", out)
	}
	// Should NOT show cascade failure
	if strings.Contains(out, "deploy") {
		t.Errorf("should not show cascade failure 'deploy', got %q", out)
	}
}

func TestFmtMs(t *testing.T) {
	tests := []struct {
		ms   int64
		want string
	}{
		{500, "500ms"},
		{1500, "1.5s"},
		{30000, "30.0s"},
		{90000, "1m30s"},
		{300000, "5m0s"},
	}
	for _, tt := range tests {
		got := fmtMs(tt.ms)
		if got != tt.want {
			t.Errorf("fmtMs(%d) = %q, want %q", tt.ms, got, tt.want)
		}
	}
}
