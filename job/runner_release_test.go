package job

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/todo"
	"github.com/amonks/incrementum/workspace"
)

func TestRunReleasesTodoStoreWorkspaceEarly(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Release todo store", todo.CreateOptions{Priority: new(todo.PriorityMedium)})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	expectedPurpose := fmt.Sprintf("todo store (job run %s)", created.ID)
	var workspaceErr error
	llmCount := 0

	_, err = Run(repoPath, created.ID, RunOptions{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{}, nil
		},
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			return nil, nil
		},
		UpdateStale: func(string) error { return nil },
		CurrentChangeEmpty: func(string) (bool, error) {
			if llmCount < 3 {
				return true, nil // Empty until the third LLM call
			}
			return false, nil // Not empty after third call writes commit message
		},
		RunLLM: func(opts AgentRunOptions) (AgentRunResult, error) {
			llmCount++
			if llmCount == 3 {
				messagePath := filepath.Join(opts.WorkspacePath, commitMessageFilename)
				if err := os.WriteFile(messagePath, []byte("feat: release store"), 0o644); err != nil {
					return AgentRunResult{}, err
				}
			}
			return AgentRunResult{SessionID: fmt.Sprintf("session-%d", llmCount), ExitCode: 0}, nil
		},
		Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
		OnStart: func(StartInfo) {
			pool, err := workspace.Open()
			if err != nil {
				workspaceErr = err
				return
			}
			items, err := pool.List(repoPath)
			if err != nil {
				workspaceErr = err
				return
			}
			for _, item := range items {
				if item.Purpose == expectedPurpose && item.Status == workspace.StatusAcquired {
					workspaceErr = fmt.Errorf("todo store workspace still acquired")
					return
				}
			}
		},
	})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}
	if workspaceErr != nil {
		t.Fatalf("%v", workspaceErr)
	}
}
