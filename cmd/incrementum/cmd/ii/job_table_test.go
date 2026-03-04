package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	jobpkg "github.com/amonks/incrementum/job"
)

func trimmedJobTable(options TableFormatOptions) string {
	return internalstrings.TrimSpace(formatJobTable(options))
}

func TestFormatJobTablePreservesAlignmentWithANSI(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Minute)

	jobs := []jobpkg.Job{
		{
			ID:        "job-1",
			TodoID:    "abc12345",
			Stage:     jobpkg.StageImplementing,
			Status:    jobpkg.StatusActive,
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
		{
			ID:          "job-2",
			TodoID:      "abd99999",
			Stage:       jobpkg.StageReviewing,
			Status:      jobpkg.StatusCompleted,
			CreatedAt:   createdAt.Add(-time.Minute),
			StartedAt:   createdAt.Add(-time.Minute),
			UpdatedAt:   now,
			CompletedAt: now,
		},
	}

	todoTitles := map[string]string{
		"abc12345": "Alpha",
		"abd99999": "Beta",
	}

	plain := formatJobTable(TableFormatOptions{
		Jobs:       jobs,
		Highlight:  func(id string, prefix int) string { return id },
		Now:        now,
		TodoTitles: todoTitles,
	})
	ansi := formatJobTable(TableFormatOptions{
		Jobs: jobs,
		Highlight: func(id string, prefix int) string {
			if prefix <= 0 || prefix > len(id) {
				return id
			}
			return "\x1b[1m\x1b[36m" + id[:prefix] + "\x1b[0m" + id[prefix:]
		},
		Now:        now,
		TodoTitles: todoTitles,
	})

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}

func TestFormatJobTableUsesProvidedJobPrefixLengths(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-5 * time.Minute)

	jobs := []jobpkg.Job{
		{
			ID:        "job-alpha",
			TodoID:    "abc12345",
			Stage:     jobpkg.StageImplementing,
			Status:    jobpkg.StatusActive,
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
		{
			ID:        "job-beta",
			TodoID:    "abd99999",
			Stage:     jobpkg.StageTesting,
			Status:    jobpkg.StatusActive,
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	jobPrefixes := map[string]int{
		"job-alpha": 2,
		"job-beta":  3,
	}

	todoTitles := map[string]string{
		"abc12345": "Alpha",
		"abd99999": "Beta",
	}

	output := formatJobTable(TableFormatOptions{
		Jobs: jobs,
		Highlight: func(id string, prefix int) string {
			return id + ":" + strconv.Itoa(prefix)
		},
		Now:              now,
		JobPrefixLengths: jobPrefixes,
		TodoTitles:       todoTitles,
	})

	if !strings.Contains(output, "job-alpha:2") {
		t.Fatalf("expected job prefix length 2, got: %q", output)
	}
	if !strings.Contains(output, "job-beta:3") {
		t.Fatalf("expected job prefix length 3, got: %q", output)
	}
}

func TestFormatJobTableUsesCompactAge(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Minute)

	jobs := []jobpkg.Job{
		{
			ID:        "job-age",
			TodoID:    "abc12345",
			Stage:     jobpkg.StageImplementing,
			Status:    jobpkg.StatusActive,
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	todoTitles := map[string]string{"abc12345": "Title"}

	output := trimmedJobTable(TableFormatOptions{
		Jobs:       jobs,
		Highlight:  func(id string, prefix int) string { return id },
		Now:        now,
		TodoTitles: todoTitles,
	})
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 10 {
		t.Fatalf("expected at least 10 columns, got: %q", lines[1])
	}

	if fields[7] != "2m" {
		t.Fatalf("expected compact age 2m, got: %s", fields[7])
	}
	if fields[8] != "2m" {
		t.Fatalf("expected compact duration 2m, got: %s", fields[8])
	}
}

func TestFormatJobTableUsesUpdatedDurationForCompleted(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Hour)
	updatedAt := now.Add(-110 * time.Minute)

	jobs := []jobpkg.Job{
		{
			ID:        "job-duration",
			TodoID:    "abc12345",
			Stage:     jobpkg.StageTesting,
			Status:    jobpkg.StatusCompleted,
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	todoTitles := map[string]string{"abc12345": "Title"}

	output := trimmedJobTable(TableFormatOptions{
		Jobs:       jobs,
		Highlight:  func(id string, prefix int) string { return id },
		Now:        now,
		TodoTitles: todoTitles,
	})
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 10 {
		t.Fatalf("expected at least 10 columns, got: %q", lines[1])
	}

	if fields[8] != "10m" {
		t.Fatalf("expected duration 10m, got: %s", fields[8])
	}
}

func TestFormatJobTableIncludesTodoTitle(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Minute)

	jobs := []jobpkg.Job{
		{
			ID:        "job-title",
			TodoID:    "abc12345",
			Stage:     jobpkg.StageImplementing,
			Status:    jobpkg.StatusActive,
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	output := formatJobTable(TableFormatOptions{
		Jobs:       jobs,
		Highlight:  func(id string, prefix int) string { return id },
		Now:        now,
		TodoTitles: map[string]string{"abc12345": "MyTitle"},
	})

	if !strings.Contains(output, "MyTitle") {
		t.Fatalf("expected todo title in output, got: %q", output)
	}
}

func TestFormatJobTableIncludesModelColumns(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Minute)

	jobs := []jobpkg.Job{
		{
			ID:                  "job-models",
			TodoID:              "abc12345",
			Stage:               jobpkg.StageImplementing,
			Status:              jobpkg.StatusActive,
			ImplementationModel: "impl-model",
			CodeReviewModel:     "review-model",
			ProjectReviewModel:  "project-model",
			CreatedAt:           createdAt,
			StartedAt:           createdAt,
			UpdatedAt:           createdAt,
		},
	}

	output := trimmedJobTable(TableFormatOptions{
		Jobs:       jobs,
		Highlight:  func(id string, prefix int) string { return id },
		Now:        now,
		TodoTitles: map[string]string{"abc12345": "Models"},
	})
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 10 {
		t.Fatalf("expected at least 10 columns, got: %q", lines[1])
	}
	if fields[4] != "impl-model" {
		t.Fatalf("expected implementation model, got: %s", fields[4])
	}
	if fields[5] != "review-model" {
		t.Fatalf("expected code review model, got: %s", fields[5])
	}
	if fields[6] != "project-model" {
		t.Fatalf("expected project review model, got: %s", fields[6])
	}
}
