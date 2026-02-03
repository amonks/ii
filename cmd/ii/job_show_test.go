package main

import (
	"strings"
	"testing"
	"time"

	jobpkg "github.com/amonks/incrementum/job"
)

func TestPrintJobDetailIncludesFeedbackAndSessions(t *testing.T) {
	startedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	job := jobpkg.Job{
		ID:       "job-123",
		TodoID:   "todo-abc",
		Stage:    jobpkg.StageImplementing,
		Status:   jobpkg.StatusActive,
		Feedback: "Please address lint failures.",
		AgentSessions: []jobpkg.AgentSession{
			{Purpose: "implement", ID: "open-1"},
			{Purpose: "review", ID: "open-2"},
		},
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}

	output := captureStdout(t, func() {
		printJobDetail(job, "Improve CLI", func(id string) string { return id }, func(id string) string { return id })
	})

	if !strings.Contains(output, "ID:      job-123") {
		t.Fatalf("expected job id in output, got: %q", output)
	}
	if !strings.Contains(output, "Todo:    todo-abc - Improve CLI") {
		t.Fatalf("expected todo line with title, got: %q", output)
	}
	if !strings.Contains(output, "Stage:   implementing") {
		t.Fatalf("expected stage in output, got: %q", output)
	}
	if !strings.Contains(output, "Status:  active") {
		t.Fatalf("expected status in output, got: %q", output)
	}
	if !strings.Contains(output, "Feedback:\nPlease address lint failures.") {
		t.Fatalf("expected feedback in output, got: %q", output)
	}
	if !strings.Contains(output, "- implement: open-1") {
		t.Fatalf("expected implement session in output, got: %q", output)
	}
	if !strings.Contains(output, "- review: open-2") {
		t.Fatalf("expected review session in output, got: %q", output)
	}
}
