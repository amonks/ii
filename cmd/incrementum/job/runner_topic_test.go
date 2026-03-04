package job

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/todo"
)

func TestRunMarksTodoInProgress(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Job topic", todo.CreateOptions{Priority: new(todo.PriorityMedium)})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	llmCount := 0

	_, err = Run(repoPath, created.ID, RunOptions{
		Now: func() time.Time { return now },
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
				if err := os.WriteFile(messagePath, []byte("feat: add topic"), 0o644); err != nil {
					return AgentRunResult{}, err
				}
			}
			return AgentRunResult{SessionID: fmt.Sprintf("session-%d", llmCount), ExitCode: 0}, nil
		},
		OnStart: func(StartInfo) {
			store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
			if err != nil {
				t.Fatalf("open todo store: %v", err)
			}
			items, err := store.Show([]string{created.ID})
			if err != nil {
				store.Release()
				t.Fatalf("show todo: %v", err)
			}
			status := items[0].Status
			store.Release()
			if status != todo.StatusInProgress {
				t.Fatalf("expected todo in progress, got %q", status)
			}
		},
	})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}

	store, err = todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	items, err := store.Show([]string{created.ID})
	if err != nil {
		store.Release()
		t.Fatalf("show todo: %v", err)
	}
	status := items[0].Status
	store.Release()
	if status != todo.StatusDone {
		t.Fatalf("expected todo done, got %q", status)
	}
}

func TestRunSkipFinalizeLeavesTodoOpen(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Skip finalize", todo.CreateOptions{Priority: new(todo.PriorityMedium)})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	now := time.Date(2026, 1, 5, 6, 7, 8, 0, time.UTC)
	llmCount := 0

	_, err = Run(repoPath, created.ID, RunOptions{
		SkipFinalize: true,
		Now:          func() time.Time { return now },
		Config:       &config.Config{Job: config.Job{TestCommands: []string{"go test ./..."}}},
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			return nil, fmt.Errorf("unexpected RunTests call")
		},
		UpdateStale: func(string) error { return nil },
		SeriesLog:   func(string) (string, error) { return "", nil },
		CurrentCommitID: func(string) (string, error) {
			return "same", nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-skip", nil
		},
		ChangeIDsForRevset: func(string, string) ([]string, error) {
			return []string{"change-skip"}, nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil
		},
		RunLLM: func(opts AgentRunOptions) (AgentRunResult, error) {
			llmCount++
			return AgentRunResult{SessionID: fmt.Sprintf("session-%d", llmCount), ExitCode: 0}, nil
		},
		OnStart: func(StartInfo) {
			store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
			if err != nil {
				t.Fatalf("open todo store: %v", err)
			}
			items, err := store.Show([]string{created.ID})
			if err != nil {
				store.Release()
				t.Fatalf("show todo: %v", err)
			}
			status := items[0].Status
			store.Release()
			if status != todo.StatusOpen {
				t.Fatalf("expected todo open, got %q", status)
			}
		},
	})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}

	store, err = todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	items, err := store.Show([]string{created.ID})
	if err != nil {
		store.Release()
		t.Fatalf("show todo: %v", err)
	}
	status := items[0].Status
	store.Release()
	if status != todo.StatusOpen {
		t.Fatalf("expected todo to remain open, got %q", status)
	}
}

func TestRunStoresModelInJobState(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Model tracking", todo.CreateOptions{Priority: new(todo.PriorityMedium)})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	now := time.Date(2026, 1, 3, 4, 5, 6, 0, time.UTC)
	llmCount := 0

	result, err := Run(repoPath, created.ID, RunOptions{
		Now: func() time.Time { return now },
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{}, nil
		},
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			return nil, nil
		},
		UpdateStale: func(string) error { return nil },
		CurrentCommitID: func(string) (string, error) {
			return "same", nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil // @ is empty
		},
		RunLLM: func(AgentRunOptions) (AgentRunResult, error) {
			llmCount++
			return AgentRunResult{SessionID: fmt.Sprintf("session-%d", llmCount), ExitCode: 0}, nil
		},
		Model: "agent-42",
	})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}

	// result.Job.Agent is the persisted state field (state.Job.Agent) that records
	// the model actually used for the run, distinct from config.Job.Model
	if result.Job.Agent != "agent-42" {
		t.Fatalf("expected model %q stored in job state (Agent field), got %q", "agent-42", result.Job.Agent)
	}

	manager, err := Open(repoPath, OpenOptions{})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}
	stored, err := manager.Find(result.Job.ID)
	if err != nil {
		t.Fatalf("find job: %v", err)
	}
	// stored.Agent is the persisted state field that records the model used
	if stored.Agent != "agent-42" {
		t.Fatalf("expected model %q stored in job state (Agent field), got %q", "agent-42", stored.Agent)
	}
}

func TestRunUsesPreloadedConfig(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Preloaded config", todo.CreateOptions{Priority: new(todo.PriorityMedium)})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	now := time.Date(2026, 1, 4, 5, 6, 7, 0, time.UTC)
	loadConfigCalled := false
	modelsUsed := []string{}
	preloaded := &config.Config{
		Job: config.Job{
			ImplementationModel: "impl-model",
			ProjectReviewModel:  "project-model",
		},
	}

	result, err := Run(repoPath, created.ID, RunOptions{
		Now:    func() time.Time { return now },
		Config: preloaded,
		LoadConfig: func(string) (*config.Config, error) {
			loadConfigCalled = true
			return nil, fmt.Errorf("unexpected LoadConfig call")
		},
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			return nil, fmt.Errorf("unexpected RunTests call")
		},
		UpdateStale: func(string) error { return nil },
		CurrentCommitID: func(string) (string, error) {
			return "same", nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil // @ is empty
		},
		RunLLM: func(opts AgentRunOptions) (AgentRunResult, error) {
			modelsUsed = append(modelsUsed, opts.Model)
			return AgentRunResult{SessionID: fmt.Sprintf("session-%d", len(modelsUsed)), ExitCode: 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}
	if loadConfigCalled {
		t.Fatal("expected LoadConfig not to be called")
	}
	// result.Job.Agent is the persisted state field that records the model used
	if result.Job.Agent != "impl-model" {
		t.Fatalf("expected model %q stored in job state (Agent field), got %q", "impl-model", result.Job.Agent)
	}
	if len(modelsUsed) < 2 {
		t.Fatalf("expected LLM calls for implement and review, got %d", len(modelsUsed))
	}
	if modelsUsed[0] != "impl-model" {
		t.Fatalf("expected implementation model %q, got %q", "impl-model", modelsUsed[0])
	}
	if modelsUsed[1] != "project-model" {
		t.Fatalf("expected review model %q, got %q", "project-model", modelsUsed[1])
	}
}
