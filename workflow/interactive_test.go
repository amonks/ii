package workflow

import "testing"

func TestInteractiveWorkflowValid(t *testing.T) {
	if err := validate(InteractiveWorkflow); err != nil {
		t.Fatalf("interactive workflow validation failed: %v", err)
	}
}

func TestInteractiveWorkflowHasTTYNode(t *testing.T) {
	found := false
	for _, n := range InteractiveWorkflow.Nodes {
		if n.Name == "implement" && n.TTY {
			found = true
		}
	}
	if !found {
		t.Error("interactive workflow should have implement node with TTY=true")
	}
}

func TestInteractiveWorkflowTerminalNode(t *testing.T) {
	edgeFrom := make(map[string]bool)
	for _, e := range InteractiveWorkflow.Edges {
		edgeFrom[e.From] = true
	}
	if edgeFrom["mark-finished"] {
		t.Error("expected mark-finished to be terminal")
	}
}
