package workflow

import "testing"

func TestHabitWorkflowValid(t *testing.T) {
	if err := validate(HabitWorkflow); err != nil {
		t.Fatalf("habit workflow validation failed: %v", err)
	}
}

func TestHabitWorkflowTerminalNode(t *testing.T) {
	edgeFrom := make(map[string]bool)
	for _, e := range HabitWorkflow.Edges {
		edgeFrom[e.From] = true
	}
	if edgeFrom["nothing-to-do"] {
		t.Error("expected nothing-to-do to be terminal")
	}
}
