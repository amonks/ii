package job

import (
	"testing"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/todo"
)

func TestResolveModelForPurposePrefersOverride(t *testing.T) {
	cfg := &config.Config{Job: config.Job{Agent: "default", ImplementationModel: "impl"}}
	item := todo.Todo{ImplementationModel: "todo-impl"}

	got := resolveModelForPurpose(cfg, "override", "implement", item)

	if got != "override" {
		t.Fatalf("expected override, got %q", got)
	}
}

func TestResolveModelForPurposeUsesTodoModels(t *testing.T) {
	cfg := &config.Config{Job: config.Job{
		Agent:               "default",
		ImplementationModel: "impl",
		CodeReviewModel:     "review",
		ProjectReviewModel:  "project",
	}}
	item := todo.Todo{
		ImplementationModel: "todo-impl",
		CodeReviewModel:     "todo-review",
		ProjectReviewModel:  "todo-project",
	}

	cases := []struct {
		name    string
		purpose string
		want    string
	}{
		{name: "implement", purpose: "implement", want: "todo-impl"},
		{name: "review", purpose: "review", want: "todo-review"},
		{name: "project-review", purpose: "project-review", want: "todo-project"},
	}

	for _, tc := range cases {
		if got := resolveModelForPurpose(cfg, "", tc.purpose, item); got != tc.want {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.want, got)
		}
	}
}

func TestResolveModelForPurposeFallsBackToAgent(t *testing.T) {
	cfg := &config.Config{Job: config.Job{Agent: "default"}}
	item := todo.Todo{}

	got := resolveModelForPurpose(cfg, "", "review", item)

	if got != "default" {
		t.Fatalf("expected default, got %q", got)
	}
}
