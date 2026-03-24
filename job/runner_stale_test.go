package job

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"monks.co/ii/internal/config"
	"monks.co/ii/todo"
)

func TestRunImplementingStageUpdatesStaleWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/repo"

	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-123", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	workspacePath := t.TempDir()
	updateCalled := false
	commitIDs := []string{"before", "after"}
	commitIndex := 0

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(path string) error {
			if path != workspacePath {
				return fmt.Errorf("unexpected workspace path %q", path)
			}
			updateCalled = true
			return errors.New("not stale")
		},
		CurrentCommitID: func(string) (string, error) {
			if commitIndex >= len(commitIDs) {
				return "", fmt.Errorf("commit id lookup exhausted")
			}
			id := commitIDs[commitIndex]
			commitIndex++
			return id, nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-stale", nil
		},
		ChangeIDsForRevset: func(string, string) ([]string, error) {
			return []string{"change-stale"}, nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return false, nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			if !updateCalled {
				return AgentRunResult{}, fmt.Errorf("expected update-stale before agent")
			}
			messagePath := filepath.Join(runOpts.WorkspacePath, commitMessageFilename)
			if err := os.WriteFile(messagePath, []byte("feat: stale"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: "oc-123", ExitCode: 0}, nil
		},
	}

	item := todo.Todo{
		ID:          "todo-123",
		Title:       "Test todo",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	result, err := runImplementingStage(manager, created, item, repoPath, workspacePath, opts, "")
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}
	if !updateCalled {
		t.Fatalf("expected update-stale to be called")
	}
	if result.Job.Stage != StageTesting {
		t.Fatalf("expected stage %q, got %q", StageTesting, result.Job.Stage)
	}
}

func TestRunImplementingStageSnapshotsWorkspaceBeforeAgent(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/repo"

	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 9, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-789", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	workspacePath := t.TempDir()
	snapshotCalled := false
	commitIDs := []string{"before", "after"}
	commitIndex := 0

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		Snapshot: func(path string) error {
			if path != workspacePath {
				return fmt.Errorf("unexpected workspace path %q", path)
			}
			snapshotCalled = true
			return nil
		},
		CurrentCommitID: func(string) (string, error) {
			if commitIndex >= len(commitIDs) {
				return "", fmt.Errorf("commit id lookup exhausted")
			}
			id := commitIDs[commitIndex]
			commitIndex++
			return id, nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-snapshot", nil
		},
		ChangeIDsForRevset: func(string, string) ([]string, error) {
			return []string{"change-snapshot"}, nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return false, nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			if !snapshotCalled {
				return AgentRunResult{}, fmt.Errorf("expected snapshot before agent")
			}
			messagePath := filepath.Join(runOpts.WorkspacePath, commitMessageFilename)
			if err := os.WriteFile(messagePath, []byte("feat: snapshot"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: "oc-789", ExitCode: 0}, nil
		},
	}

	item := todo.Todo{
		ID:          "todo-789",
		Title:       "Snapshot todo",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	result, err := runImplementingStage(manager, created, item, repoPath, workspacePath, opts, "")
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}
	if !snapshotCalled {
		t.Fatalf("expected snapshot to be called")
	}
	if result.Job.Stage != StageTesting {
		t.Fatalf("expected stage %q, got %q", StageTesting, result.Job.Stage)
	}
}

func TestRunReviewingStageUpdatesStaleWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/repo-review"

	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 11, 10, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-456", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	workspacePath := t.TempDir()
	updateCalled := false
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(path string) error {
			if path != workspacePath {
				return fmt.Errorf("unexpected workspace path %q", path)
			}
			updateCalled = true
			return errors.New("not stale")
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-review", nil
		},
		ChangeIDsForRevset: func(string, string) ([]string, error) {
			return []string{"change-review"}, nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			if !updateCalled {
				return AgentRunResult{}, fmt.Errorf("expected update-stale before agent")
			}
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: "oc-456", ExitCode: 0}, nil
		},
		Config: &config.Config{Job: config.Job{TestCommands: []string{"go test ./..."}}},
	}

	item := todo.Todo{
		ID:          "todo-456",
		Title:       "Review todo",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitMessage := "fix: stale review"
	result, err := runReviewingStage(manager, created, item, repoPath, workspacePath, opts, commitMessage, reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}
	if !updateCalled {
		t.Fatalf("expected update-stale to be called")
	}
	if result.Job.Stage != StageCommitting {
		t.Fatalf("expected stage %q, got %q", StageCommitting, result.Job.Stage)
	}
}

func TestRunReviewingStageSnapshotsWorkspaceBeforeAgent(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/repo-review"

	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 13, 10, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-987", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	workspacePath := t.TempDir()
	snapshotCalled := false
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		Snapshot: func(path string) error {
			if path != workspacePath {
				return fmt.Errorf("unexpected workspace path %q", path)
			}
			snapshotCalled = true
			return nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-review-snapshot", nil
		},
		ChangeIDsForRevset: func(string, string) ([]string, error) {
			return []string{"change-review-snapshot"}, nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			if !snapshotCalled {
				return AgentRunResult{}, fmt.Errorf("expected snapshot before agent")
			}
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: "oc-987", ExitCode: 0}, nil
		},
		Config: &config.Config{Job: config.Job{TestCommands: []string{"go test ./..."}}},
	}

	item := todo.Todo{
		ID:          "todo-987",
		Title:       "Review snapshot todo",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitMessage := "fix: snapshot review"
	result, err := runReviewingStage(manager, created, item, repoPath, workspacePath, opts, commitMessage, reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}
	if !snapshotCalled {
		t.Fatalf("expected snapshot to be called")
	}
	if result.Job.Stage != StageCommitting {
		t.Fatalf("expected stage %q, got %q", StageCommitting, result.Job.Stage)
	}
}
