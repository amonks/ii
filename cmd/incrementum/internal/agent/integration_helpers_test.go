package agent_test

import (
	"testing"

	"monks.co/incrementum/internal/llm"
)

func requireModel(t *testing.T, modelID string) llm.Model {
	t.Helper()
	return requireModelFromRepoConfig(t, modelID)
}
