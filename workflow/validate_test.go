package workflow

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		wf      Workflow
		wantErr string
	}{
		{
			name:    "empty name",
			wf:      Workflow{},
			wantErr: "workflow name is required",
		},
		{
			name:    "empty start",
			wf:      Workflow{Name: "test"},
			wantErr: "start node is required",
		},
		{
			name:    "no nodes",
			wf:      Workflow{Name: "test", Start: "a"},
			wantErr: "at least one node is required",
		},
		{
			name: "start not found",
			wf: Workflow{
				Name:  "test",
				Start: "missing",
				Nodes: []Node{{Name: "a"}},
			},
			wantErr: `start node "missing" not found`,
		},
		{
			name: "duplicate node",
			wf: Workflow{
				Name:  "test",
				Start: "a",
				Nodes: []Node{{Name: "a"}, {Name: "a"}},
			},
			wantErr: "duplicate node name: a",
		},
		{
			name: "edge unknown source",
			wf: Workflow{
				Name:  "test",
				Start: "a",
				Nodes: []Node{{Name: "a"}},
				Edges: []Edge{{From: "missing", To: "a"}},
			},
			wantErr: "unknown source node: missing",
		},
		{
			name: "edge unknown destination",
			wf: Workflow{
				Name:  "test",
				Start: "a",
				Nodes: []Node{{Name: "a"}},
				Edges: []Edge{{From: "a", To: "missing"}},
			},
			wantErr: "unknown destination node: missing",
		},
		{
			name: "valid workflow",
			wf: Workflow{
				Name:  "test",
				Start: "a",
				Nodes: []Node{{Name: "a"}, {Name: "b"}},
				Edges: []Edge{{From: "a", To: "b"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.wf)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
