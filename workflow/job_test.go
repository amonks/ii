package workflow

import "testing"

func TestJobWorkflowValid(t *testing.T) {
	if err := validate(JobWorkflow); err != nil {
		t.Fatalf("job workflow validation failed: %v", err)
	}
}

func TestJobWorkflowHasRequiredInput(t *testing.T) {
	found := false
	for _, input := range JobWorkflow.Inputs {
		if input.Name == "todo-id" && input.Required {
			found = true
		}
	}
	if !found {
		t.Error("job workflow should have required 'todo-id' input")
	}
}

func TestJobWorkflowTerminalNodes(t *testing.T) {
	edgeFrom := make(map[string]bool)
	for _, e := range JobWorkflow.Edges {
		edgeFrom[e.From] = true
	}

	// mark-finished and reopen-todo should be terminal (no outgoing edges).
	terminals := []string{"mark-finished", "reopen-todo"}
	for _, name := range terminals {
		if edgeFrom[name] {
			t.Errorf("expected %s to be terminal (no outgoing edges)", name)
		}
	}
}
