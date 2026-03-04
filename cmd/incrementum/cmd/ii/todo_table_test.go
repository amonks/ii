package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/todo"
)

func TestFormatTodoTablePreservesAlignmentWithANSI(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	todos := []todo.Todo{
		{
			ID:        "abc123",
			Priority:  1,
			Type:      todo.TodoType("task"),
			Status:    todo.StatusOpen,
			Title:     "First item",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "abd456",
			Priority:  2,
			Type:      todo.TodoType("bug"),
			Status:    todo.StatusInProgress,
			Title:     "Second item",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	prefixLengths := todoIDPrefixLengths(todos)
	plain := formatTodoTable(todos, prefixLengths, func(id string, prefix int) string { return id }, now)
	ansi := formatTodoTable(todos, prefixLengths, func(id string, prefix int) string {
		if prefix <= 0 || prefix > len(id) {
			return id
		}
		return "\x1b[1m\x1b[36m" + id[:prefix] + "\x1b[0m" + id[prefix:]
	}, now)

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}

func TestFormatTodoTableUsesProvidedPrefixLengths(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	todos := []todo.Todo{
		{
			ID:        "r1234567",
			Priority:  2,
			Type:      todo.TodoType("task"),
			Status:    todo.StatusOpen,
			Title:     "Only listed",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	prefixLengths := map[string]int{"r1234567": 2}
	output := formatTodoTable(todos, prefixLengths, func(id string, prefix int) string {
		return fmt.Sprintf("%s:%d", id, prefix)
	}, now)

	if !strings.Contains(output, "r1234567:2") {
		t.Fatalf("expected table to use provided prefix length, got:\n%s", output)
	}
}

func TestFormatTodoTableShowsAge(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	todos := []todo.Todo{
		{
			ID:        "abc123",
			Priority:  2,
			Type:      todo.TodoType("task"),
			Status:    todo.StatusClosed,
			Title:     "Time check",
			CreatedAt: now.Add(-2 * time.Hour),
			UpdatedAt: now.Add(-45 * time.Minute),
		},
	}

	output := formatTodoTable(todos, nil, func(id string, prefix int) string { return id }, now)

	if !strings.Contains(output, "2h") {
		t.Fatalf("expected age in output, got:\n%s", output)
	}
	if !strings.Contains(output, "45m") {
		t.Fatalf("expected updated age in output, got:\n%s", output)
	}
	if !strings.Contains(output, "DURATION") {
		t.Fatalf("expected duration column present, got:\n%s", output)
	}
	if !strings.Contains(output, "UPDATED") {
		t.Fatalf("expected updated column present, got:\n%s", output)
	}
}

func TestFormatTodoTableShowsSecondsAge(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 30, 0, time.UTC)
	todos := []todo.Todo{
		{
			ID:        "def456",
			Priority:  1,
			Type:      todo.TodoType("task"),
			Status:    todo.StatusOpen,
			Title:     "Seconds check",
			CreatedAt: now.Add(-45 * time.Second),
			UpdatedAt: now.Add(-45 * time.Second),
		},
	}

	output := formatTodoTable(todos, nil, func(id string, prefix int) string { return id }, now)

	if !strings.Contains(output, "45s") {
		t.Fatalf("expected seconds age in output, got:\n%s", output)
	}
}

func TestFormatTodoTableShowsDuration(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	startedInProgress := now.Add(-45 * time.Minute)
	startedDone := now.Add(-3 * time.Hour)
	completedDone := now.Add(-2 * time.Hour)

	todos := []todo.Todo{
		{
			ID:        "abc123",
			Priority:  2,
			Type:      todo.TodoType("task"),
			Status:    todo.StatusInProgress,
			Title:     "In progress",
			CreatedAt: now,
			UpdatedAt: now,
			StartedAt: &startedInProgress,
		},
		{
			ID:          "def456",
			Priority:    1,
			Type:        todo.TodoType("task"),
			Status:      todo.StatusDone,
			Title:       "Done",
			CreatedAt:   now,
			UpdatedAt:   now,
			StartedAt:   &startedDone,
			CompletedAt: &completedDone,
		},
	}

	output := formatTodoTable(todos, nil, func(id string, prefix int) string { return id }, now)

	if !strings.Contains(output, "45m") {
		t.Fatalf("expected in-progress duration in output, got:\n%s", output)
	}
	if !strings.Contains(output, "1h") {
		t.Fatalf("expected done duration in output, got:\n%s", output)
	}
}
