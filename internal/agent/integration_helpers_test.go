package agent_test

import (
	"testing"

	"github.com/amonks/incrementum/internal/llm"
)

func requireModel(t *testing.T, modelID string) llm.Model {
	t.Helper()
	return requireModelFromRepoConfig(t, modelID)
}
