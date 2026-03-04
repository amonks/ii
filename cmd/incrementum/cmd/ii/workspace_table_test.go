package main

import (
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/ui"
	"github.com/amonks/incrementum/workspace"
)

func TestFormatWorkspaceTablePreservesAlignmentWithANSI(t *testing.T) {
	items := []workspace.Info{
		{
			Name:    "ws-001",
			Path:    "/tmp/ws-001",
			Purpose: "feature work",
			Status:  workspace.StatusAvailable,
		},
		{
			Name:    "ws-010",
			Path:    "/tmp/ws-010",
			Purpose: "bugfix",
			Status:  workspace.StatusAcquired,
		},
	}

	now := time.Date(2026, 1, 23, 1, 0, 0, 0, time.UTC)

	plain := formatWorkspaceTable(items, func(value string) string { return value }, now)
	ansi := formatWorkspaceTable(items, func(value string) string {
		return "\x1b[1m\x1b[36m" + value + "\x1b[0m"
	}, now)

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}

func TestFormatWorkspaceTableTruncatesLongPaths(t *testing.T) {
	longPath := "/tmp/" + strings.Repeat("a", 60)
	items := []workspace.Info{
		{
			Name:    "ws-002",
			Path:    longPath,
			Purpose: "refactor",
			Status:  workspace.StatusAvailable,
		},
	}

	now := time.Date(2026, 1, 23, 1, 0, 0, 0, time.UTC)

	output := formatWorkspaceTable(items, nil, now)
	expected := longPath[:47] + "..."
	if !strings.Contains(output, expected) {
		t.Fatalf("expected truncated path %q in output: %s", expected, output)
	}
	if strings.Contains(output, longPath) {
		t.Fatalf("expected long path to be truncated, got: %s", output)
	}
}

func TestFormatWorkspaceTableShowsAcquiredAge(t *testing.T) {
	now := time.Date(2026, 1, 23, 2, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Hour)
	items := []workspace.Info{
		{
			Name:      "ws-003",
			Path:      "/tmp/ws-003",
			Purpose:   "age-check",
			Status:    workspace.StatusAcquired,
			CreatedAt: createdAt,
		},
	}

	output := formatWorkspaceTable(items, nil, now)
	expected := ui.FormatDurationShort(now.Sub(createdAt))
	if !strings.Contains(output, expected) {
		t.Fatalf("expected acquired age %q in output: %s", expected, output)
	}
}

func TestFormatWorkspaceTableShowsDurationForActiveAndAvailable(t *testing.T) {
	now := time.Date(2026, 1, 23, 4, 0, 0, 0, time.UTC)
	createdAt := now.Add(-90 * time.Minute)
	updatedAt := now.Add(-30 * time.Minute)
	items := []workspace.Info{
		{
			Name:      "ws-005",
			Path:      "/tmp/ws-005",
			Purpose:   "active",
			Status:    workspace.StatusAcquired,
			CreatedAt: createdAt,
		},
		{
			Name:      "ws-006",
			Path:      "/tmp/ws-006",
			Purpose:   "available",
			Status:    workspace.StatusAvailable,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	output := formatWorkspaceTable(items, nil, now)
	activeDuration := ui.FormatDurationShort(now.Sub(createdAt))
	availableDuration := ui.FormatDurationShort(updatedAt.Sub(createdAt))
	if !strings.Contains(output, activeDuration) {
		t.Fatalf("expected active duration %q in output: %s", activeDuration, output)
	}
	if !strings.Contains(output, availableDuration) {
		t.Fatalf("expected available duration %q in output: %s", availableDuration, output)
	}
}

func TestFormatWorkspaceTableShowsRevision(t *testing.T) {
	now := time.Date(2026, 1, 23, 3, 0, 0, 0, time.UTC)
	items := []workspace.Info{
		{
			Name:    "ws-004",
			Path:    "/tmp/ws-004",
			Purpose: "rev-check",
			Status:  workspace.StatusAcquired,
			Rev:     "main~2",
		},
	}

	output := formatWorkspaceTable(items, nil, now)
	if !strings.Contains(output, "main~2") {
		t.Fatalf("expected revision %q in output: %s", "main~2", output)
	}
}

func TestFilterWorkspaceListDefaultsToAvailableAndAcquired(t *testing.T) {
	items := []workspace.Info{
		{Name: "ws-001", Status: workspace.StatusAvailable},
		{Name: "ws-002", Status: workspace.StatusAcquired},
	}

	filtered := filterWorkspaceList(items, false)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(filtered))
	}
	if filtered[0].Name != "ws-001" {
		t.Fatalf("expected ws-001, got %q", filtered[0].Name)
	}
	if filtered[1].Name != "ws-002" {
		t.Fatalf("expected ws-002, got %q", filtered[1].Name)
	}
}

func TestFilterWorkspaceListWithAll(t *testing.T) {
	items := []workspace.Info{
		{Name: "ws-001", Status: workspace.StatusAvailable},
		{Name: "ws-002", Status: workspace.StatusAcquired},
	}

	filtered := filterWorkspaceList(items, true)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(filtered))
	}
}
