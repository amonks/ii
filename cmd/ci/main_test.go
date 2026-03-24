package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
