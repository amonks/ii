package todoenv

import (
	"testing"

	"github.com/amonks/incrementum/todo"
)

func TestDefaultStatusUsesOpenByDefault(t *testing.T) {
	t.Setenv(ProposerEnvVar, "false")

	if got := DefaultStatus(); got != todo.StatusOpen {
		t.Fatalf("expected open status, got %q", got)
	}
}

func TestDefaultStatusUsesProposedWhenEnabled(t *testing.T) {
	t.Setenv(ProposerEnvVar, "true")

	if got := DefaultStatus(); got != todo.StatusProposed {
		t.Fatalf("expected proposed status, got %q", got)
	}
}
