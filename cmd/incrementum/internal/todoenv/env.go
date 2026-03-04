package todoenv

import (
	"os"
	"strings"

	"github.com/amonks/incrementum/todo"
)

// ProposerEnvVar is the environment variable that switches todo defaults.
const ProposerEnvVar = "INCREMENTUM_TODO_PROPOSER"

// DefaultStatus returns the todo status implied by the environment.
func DefaultStatus() todo.Status {
	if strings.EqualFold(os.Getenv(ProposerEnvVar), "true") {
		return todo.StatusProposed
	}
	return todo.StatusOpen
}
