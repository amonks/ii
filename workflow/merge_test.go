package workflow

import "testing"

func TestMergeWorkflowValid(t *testing.T) {
	if err := validate(MergeWorkflow); err != nil {
		t.Fatalf("merge workflow validation failed: %v", err)
	}
}

func TestMergeWorkflowInputs(t *testing.T) {
	found := false
	hasDefault := false
	for _, input := range MergeWorkflow.Inputs {
		if input.Name == "change-id" && input.Required {
			found = true
		}
		if input.Name == "target" && input.Default == "main" {
			hasDefault = true
		}
	}
	if !found {
		t.Error("merge workflow should have required 'change-id' input")
	}
	if !hasDefault {
		t.Error("merge workflow should have 'target' input with default 'main'")
	}
}

func TestMergeWorkflowTerminalNode(t *testing.T) {
	edgeFrom := make(map[string]bool)
	for _, e := range MergeWorkflow.Edges {
		edgeFrom[e.From] = true
	}
	if edgeFrom["verify-clean"] {
		t.Error("expected verify-clean to be terminal (no outgoing edges)")
	}
}
