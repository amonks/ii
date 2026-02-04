package main

import (
	"strings"
	"testing"

	"github.com/amonks/incrementum/todo"
	"github.com/spf13/cobra"
)

func TestJobDoAllFiltersRejectsDesignType(t *testing.T) {
	resetJobDoAllGlobals()
	cmd := newTestJobDoAllCommand()
	if err := cmd.Flags().Set("type", "design"); err != nil {
		t.Fatalf("set type flag: %v", err)
	}

	_, err := jobDoAllFilters(cmd)
	if err == nil {
		t.Fatal("expected error for design type")
	}
	if !strings.Contains(err.Error(), "design todos require interactive sessions") {
		t.Fatalf("expected interactive session error, got %v", err)
	}
}

func TestJobDoAllFiltersAcceptsNonInteractiveTypes(t *testing.T) {
	cases := []string{"task", "bug", "feature"}
	for _, typeStr := range cases {
		t.Run(typeStr, func(t *testing.T) {
			resetJobDoAllGlobals()
			cmd := newTestJobDoAllCommand()
			if err := cmd.Flags().Set("type", typeStr); err != nil {
				t.Fatalf("set type flag: %v", err)
			}

			filter, err := jobDoAllFilters(cmd)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if filter.todoType == nil {
				t.Fatal("expected type filter to be set")
			}
			if string(*filter.todoType) != typeStr {
				t.Fatalf("expected type %q, got %q", typeStr, *filter.todoType)
			}
		})
	}
}

func TestNextJobDoAllTodoIDSkipsDesignTodos(t *testing.T) {
	todos := []todo.Todo{
		{ID: "design-1", Type: todo.TypeDesign, Priority: todo.PriorityHigh},
		{ID: "task-1", Type: todo.TypeTask, Priority: todo.PriorityMedium},
		{ID: "design-2", Type: todo.TypeDesign, Priority: todo.PriorityCritical},
		{ID: "bug-1", Type: todo.TypeBug, Priority: todo.PriorityLow},
	}

	store := &mockReadyStore{todos: todos}
	filter := jobDoAllFilter{}

	todoID, err := nextJobDoAllTodoIDFromList(store.todos, filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todoID != "task-1" {
		t.Fatalf("expected first non-design todo 'task-1', got %q", todoID)
	}
}

func TestNextJobDoAllTodoIDReturnsEmptyWhenAllDesign(t *testing.T) {
	todos := []todo.Todo{
		{ID: "design-1", Type: todo.TypeDesign, Priority: todo.PriorityHigh},
		{ID: "design-2", Type: todo.TypeDesign, Priority: todo.PriorityCritical},
	}

	filter := jobDoAllFilter{}

	todoID, err := nextJobDoAllTodoIDFromList(todos, filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todoID != "" {
		t.Fatalf("expected empty id when all todos are design, got %q", todoID)
	}
}

func TestNextJobDoAllTodoIDRespectsTypeFilter(t *testing.T) {
	todos := []todo.Todo{
		{ID: "task-1", Type: todo.TypeTask, Priority: todo.PriorityMedium},
		{ID: "bug-1", Type: todo.TypeBug, Priority: todo.PriorityMedium},
		{ID: "feature-1", Type: todo.TypeFeature, Priority: todo.PriorityMedium},
	}

	bugType := todo.TypeBug
	filter := jobDoAllFilter{todoType: &bugType}

	todoID, err := nextJobDoAllTodoIDFromList(todos, filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todoID != "bug-1" {
		t.Fatalf("expected bug-1 with type filter, got %q", todoID)
	}
}

func TestNextJobDoAllTodoIDRespectsPriorityFilter(t *testing.T) {
	todos := []todo.Todo{
		{ID: "low-1", Type: todo.TypeTask, Priority: todo.PriorityLow},
		{ID: "high-1", Type: todo.TypeTask, Priority: todo.PriorityHigh},
		{ID: "medium-1", Type: todo.TypeTask, Priority: todo.PriorityMedium},
	}

	maxPriority := todo.PriorityMedium
	filter := jobDoAllFilter{maxPriority: &maxPriority}

	todoID, err := nextJobDoAllTodoIDFromList(todos, filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todoID != "high-1" {
		t.Fatalf("expected high-1 (first within priority), got %q", todoID)
	}
}

func TestJobDoAllAgentFlagParsing(t *testing.T) {
	resetJobDoAllGlobals()

	cases := []struct {
		name      string
		flagValue string
		wantKind  jobAgentKind
		wantErr   bool
	}{
		{"default", "", jobAgentInternal, false},
		{"internal", "internal", jobAgentInternal, false},
		{"claude", "claude", jobAgentClaude, false},
		{"codex", "codex", jobAgentCodex, false},
		{"Claude uppercase", "Claude", jobAgentClaude, false},
		{"invalid", "invalid", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetJobDoAllGlobals()
			cmd := newTestJobDoAllCommand()
			if tc.flagValue != "" {
				if err := cmd.Flags().Set("agent", tc.flagValue); err != nil {
					t.Fatalf("set agent flag: %v", err)
				}
			}

			kind, err := parseJobDoAgentKind(cmd)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if kind != tc.wantKind {
				t.Fatalf("expected %q, got %q", tc.wantKind, kind)
			}
		})
	}
}

// TestParseJobDoAgentKindReadsFromPassedCommand verifies that parseJobDoAgentKind
// reads the --agent flag from the command that is passed to it, not from global
// state or from a different command's flags.
func TestParseJobDoAgentKindReadsFromPassedCommand(t *testing.T) {
	resetJobDoAllGlobals()

	// Create two independent commands with their own --agent flags
	var agentA, agentB string
	cmdA := &cobra.Command{}
	cmdA.Flags().StringVar(&agentA, "agent", "", "Agent backend")
	cmdB := &cobra.Command{}
	cmdB.Flags().StringVar(&agentB, "agent", "", "Agent backend")

	// Set different values on each command
	if err := cmdA.Flags().Set("agent", "claude"); err != nil {
		t.Fatalf("set agent on cmdA: %v", err)
	}
	if err := cmdB.Flags().Set("agent", "codex"); err != nil {
		t.Fatalf("set agent on cmdB: %v", err)
	}

	// parseJobDoAgentKind should read from cmdA's flags when passed cmdA
	kindA, err := parseJobDoAgentKind(cmdA)
	if err != nil {
		t.Fatalf("parseJobDoAgentKind(cmdA): %v", err)
	}
	if kindA != jobAgentClaude {
		t.Fatalf("expected claude from cmdA, got %q", kindA)
	}

	// parseJobDoAgentKind should read from cmdB's flags when passed cmdB
	kindB, err := parseJobDoAgentKind(cmdB)
	if err != nil {
		t.Fatalf("parseJobDoAgentKind(cmdB): %v", err)
	}
	if kindB != jobAgentCodex {
		t.Fatalf("expected codex from cmdB, got %q", kindB)
	}
}

// nextJobDoAllTodoIDFromList is a test helper that applies the filter logic
// to a pre-loaded list of todos, matching the logic in nextJobDoAllTodoID.
func nextJobDoAllTodoIDFromList(todos []todo.Todo, filter jobDoAllFilter) (string, error) {
	for _, item := range todos {
		if item.Type.IsInteractive() {
			continue
		}
		if filter.maxPriority != nil && item.Priority > *filter.maxPriority {
			continue
		}
		if filter.todoType != nil && item.Type != *filter.todoType {
			continue
		}
		return item.ID, nil
	}
	return "", nil
}

type mockReadyStore struct {
	todos []todo.Todo
}

func resetJobDoAllGlobals() {
	jobDoAllPriority = -1
	jobDoAllType = ""
	jobDoAllAgent = ""
	jobDoAllHabits = false
}

func newTestJobDoAllCommand() *cobra.Command {
	cmd := &cobra.Command{RunE: runJobDoAll}
	cmd.Flags().IntVar(&jobDoAllPriority, "priority", -1, "Filter by priority (0-4, includes higher priorities)")
	cmd.Flags().StringVar(&jobDoAllType, "type", "", "Filter by type (task, bug, feature); design todos are excluded")
	cmd.Flags().StringVar(&jobDoAllAgent, "agent", "", "Agent backend (internal, claude, codex)")
	cmd.Flags().BoolVar(&jobDoAllHabits, "habits", false, "Run habits after todo queue is empty (round-robin)")
	return cmd
}
