package job

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/todo"
)

func TestRunImplementingStage_MissingCommitMessageRetries(t *testing.T) {
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	current, err := manager.Create("todo-1", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-1",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	commitCalls := 0
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			commitCalls++
			if commitCalls == 1 {
				return "before", nil
			}
			return "after", nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-123", nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return false, nil
		},
		RunLLM: func(AgentRunOptions) (AgentRunResult, error) {
			return AgentRunResult{SessionID: "ses-123", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, current, item, repoPath, repoPath, opts, "")
	if err != nil {
		t.Fatalf("expected no error for missing commit message (should retry), got %v", err)
	}
	// Should stay in implementing stage with feedback
	if result.Job.Stage != StageImplementing {
		t.Fatalf("expected stage %q, got %q", StageImplementing, result.Job.Stage)
	}
	if result.Job.Feedback == "" {
		t.Fatal("expected feedback to be set")
	}
	if !strings.Contains(result.Job.Feedback, "commit message") {
		t.Fatalf("expected feedback to mention commit message, got %q", result.Job.Feedback)
	}
	if !strings.Contains(result.Job.Feedback, commitMessageFilename) {
		t.Fatalf("expected feedback to mention commit message file, got %q", result.Job.Feedback)
	}
	// Changed should be true so we don't go to project review
	if !result.Changed {
		t.Fatal("expected changed to be true")
	}
}

func TestRunImplementingStageFailedAgentRestoresRetriesAndReportsContext(t *testing.T) {
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 6, 0, time.UTC)
	current, err := manager.Create("todo-restore", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-restore",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	commitCalls := 0
	restoreCalls := 0
	restoreCommit := ""
	runCalls := 0
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			commitCalls++
			switch commitCalls {
			case 1:
				return "before", nil
			case 2:
				return "after-first", nil
			default:
				return "after-second", nil
			}
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-restore", nil
		},
		RunLLM: func(AgentRunOptions) (AgentRunResult, error) {
			runCalls++
			if runCalls == 1 {
				return AgentRunResult{
					SessionID: "ses-789",
					ExitCode:  -1,
				}, nil
			}
			return AgentRunResult{
				SessionID: "ses-790",
				ExitCode:  -1,
			}, nil
		},
		RestoreWorkspace: func(_ string, commitID string) error {
			restoreCalls++
			restoreCommit = commitID
			return nil
		},
		Model: "claude-haiku-4-5",
	}

	_, err = runImplementingStage(manager, current, item, repoPath, repoPath, opts, "")
	if err == nil {
		t.Fatal("expected agent failure error")
	}
	if restoreCalls != 2 {
		t.Fatalf("expected restore to be called twice, got %d", restoreCalls)
	}
	if restoreCommit != "before" {
		t.Fatalf("expected restore commit to be before, got %q", restoreCommit)
	}
	message := err.Error()
	if !strings.Contains(message, "agent implement failed with exit code -1") {
		t.Fatalf("expected agent failure message with exit code, got %v", message)
	}
	if !strings.Contains(message, "process did not exit cleanly") {
		t.Fatalf("expected unclean exit context, got %v", message)
	}
	if !strings.Contains(message, "session ses-790") {
		t.Fatalf("expected session context, got %v", message)
	}
	if !strings.Contains(message, "model \"claude-haiku-4-5\"") {
		t.Fatalf("expected model context, got %v", message)
	}
	if !strings.Contains(message, "prompt prompt-implementation.tmpl") {
		t.Fatalf("expected prompt context, got %v", message)
	}
	// Note: run and serve commands are no longer included in the unified error message
	if !strings.Contains(message, "before before") || !strings.Contains(message, "after after-second") {
		t.Fatalf("expected commit context, got %v", message)
	}
	if !strings.Contains(message, "restored before") {
		t.Fatalf("expected restore context, got %v", message)
	}
	if !strings.Contains(message, "retry 1") {
		t.Fatalf("expected retry context, got %v", message)
	}
}

func TestRunImplementingStageRetriesAgentAfterRestore(t *testing.T) {
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 6, 30, time.UTC)
	current, err := manager.Create("todo-retry", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-retry",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	commitCalls := 0
	restoreCalls := 0
	runCalls := 0
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			commitCalls++
			switch commitCalls {
			case 1:
				return "before", nil
			case 2:
				return "after-bad", nil
			default:
				return "before", nil
			}
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-retry", nil
		},
		RunLLM: func(AgentRunOptions) (AgentRunResult, error) {
			runCalls++
			if runCalls == 1 {
				return AgentRunResult{SessionID: "ses-1", ExitCode: -1}, nil
			}
			return AgentRunResult{SessionID: "ses-2", ExitCode: 0}, nil
		},
		RestoreWorkspace: func(string, string) error {
			restoreCalls++
			return nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil // @ is empty after retry and restore
		},
	}

	result, err := runImplementingStage(manager, current, item, repoPath, repoPath, opts, "")
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if runCalls != 2 {
		t.Fatalf("expected agent to run twice, got %d", runCalls)
	}
	if restoreCalls != 1 {
		t.Fatalf("expected restore to be called once, got %d", restoreCalls)
	}
	if result.Changed {
		t.Fatalf("expected no change after retry")
	}
	if result.Job.Stage != StageReviewing {
		t.Fatalf("expected stage %q, got %q", StageReviewing, result.Job.Stage)
	}
}

func TestRunImplementingStageTreatsEmptyChangeAsNoChange(t *testing.T) {
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	current, err := manager.Create("todo-2", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-2",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	messagePath := filepath.Join(repoPath, commitMessageFilename)
	if err := os.WriteFile(messagePath, []byte("feat: example\n"), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	commitCalls := 0
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			commitCalls++
			if commitCalls == 1 {
				return "before", nil
			}
			return "after", nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-456", nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil
		},
		RunLLM: func(AgentRunOptions) (AgentRunResult, error) {
			return AgentRunResult{SessionID: "ses-456", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, current, item, repoPath, repoPath, opts, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Changed {
		t.Fatalf("expected no change, got changed")
	}
	if _, err := os.Stat(messagePath); !os.IsNotExist(err) {
		t.Fatalf("expected commit message to be deleted")
	}
}

func TestRunImplementingStageTreatsEmptyChangeAsNoChangeAfterCommit(t *testing.T) {
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 6, 0, time.UTC)
	current, err := manager.Create("todo-3", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-3",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	messagePath := filepath.Join(repoPath, commitMessageFilename)
	if err := os.WriteFile(messagePath, []byte("feat: example\n"), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	commitCalls := 0
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			commitCalls++
			if commitCalls == 1 {
				return "before", nil
			}
			return "after", nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-789", nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil
		},
		RunLLM: func(AgentRunOptions) (AgentRunResult, error) {
			return AgentRunResult{SessionID: "ses-789", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, current, item, repoPath, repoPath, opts, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Changed {
		t.Fatalf("expected no change, got changed")
	}
	if _, err := os.Stat(messagePath); !os.IsNotExist(err) {
		t.Fatalf("expected commit message to be deleted")
	}
}

func TestRunImplementingStageDetectsUncommittedWorkFromPreviousRun(t *testing.T) {
	// This tests the scenario where a previous job left uncommitted work in @.
	// The LLM makes no changes (commit ID doesn't change), but @ is not empty
	// because of work from the previous run.
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 8, 0, time.UTC)
	current, err := manager.Create("todo-prior", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-prior",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	// Write a commit message file (simulating that the previous run created it)
	messagePath := filepath.Join(repoPath, commitMessageFilename)
	if err := os.WriteFile(messagePath, []byte("feat: work from previous run\n"), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	opts := RunOptions{
		Now: func() time.Time { return now },
		// Commit ID does NOT change during this run (LLM made no changes)
		CurrentCommitID: func(string) (string, error) {
			return "same-commit", nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-prior", nil
		},
		// But @ is NOT empty because of work from a previous run
		CurrentChangeEmpty: func(string) (bool, error) {
			return false, nil
		},
		RunLLM: func(AgentRunOptions) (AgentRunResult, error) {
			return AgentRunResult{SessionID: "ses-prior", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, current, item, repoPath, repoPath, opts, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Should detect the uncommitted work and flag it as changed
	if !result.Changed {
		t.Fatalf("expected changed=true because @ is not empty")
	}
	if result.Job.Stage != StageTesting {
		t.Fatalf("expected stage %q, got %q", StageTesting, result.Job.Stage)
	}
	if result.CommitMessage != "feat: work from previous run" {
		t.Fatalf("expected commit message from file, got %q", result.CommitMessage)
	}
}
func TestRunImplementingStageIncludesErrorInFailureMessage(t *testing.T) {
	// This test verifies that non-context-overflow errors are properly included in
	// the failure message without triggering retry logic.
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 7, 0, time.UTC)
	current, err := manager.Create("todo-error", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-error",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			return "same-commit", nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-error", nil
		},
		RunLLM: func(AgentRunOptions) (AgentRunResult, error) {
			return AgentRunResult{
				SessionID: "ses-error",
				ExitCode:  1,
				Error:     "some other error that should fail",
			}, nil
		},
		Model: "test-model",
	}

	_, err = runImplementingStage(manager, current, item, repoPath, repoPath, opts, "")
	if err == nil {
		t.Fatal("expected error for failed agent")
	}
	message := err.Error()
	// The error message should now include the actual error reason and exit code
	if !strings.Contains(message, "agent implement failed with exit code 1") {
		t.Fatalf("expected agent failure message with exit code, got %v", message)
	}
	if !strings.Contains(message, "error: some other error that should fail") {
		t.Fatalf("expected error reason in message, got %v", message)
	}
	if !strings.Contains(message, "session ses-error") {
		t.Fatalf("expected session context, got %v", message)
	}
}

func TestRunImplementingStageRetriesOnContextOverflow(t *testing.T) {
	// This test verifies that context overflow errors trigger a retry without
	// restoring the workspace.
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 7, 0, time.UTC)
	current, err := manager.Create("todo-overflow", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-overflow",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	runCalls := 0
	restoreCalls := 0
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			return "same-commit", nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-overflow", nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil // No changes after retry
		},
		RunLLM: func(AgentRunOptions) (AgentRunResult, error) {
			runCalls++
			if runCalls == 1 {
				// First call: context overflow
				return AgentRunResult{
					SessionID: "ses-overflow-1",
					ExitCode:  1,
					Error:     "context overflow: max tokens reached",
				}, nil
			}
			// Second call: success
			return AgentRunResult{
				SessionID: "ses-overflow-2",
				ExitCode:  0,
			}, nil
		},
		RestoreWorkspace: func(string, string) error {
			restoreCalls++
			return nil
		},
		Model: "test-model",
	}

	result, err := runImplementingStage(manager, current, item, repoPath, repoPath, opts, "")
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if runCalls != 2 {
		t.Fatalf("expected agent to run twice, got %d", runCalls)
	}
	// Important: workspace should NOT be restored for context overflow
	if restoreCalls != 0 {
		t.Fatalf("expected no workspace restore for context overflow, got %d restore calls", restoreCalls)
	}
	if result.Changed {
		t.Fatalf("expected no change (workspace empty)")
	}
}


func TestRunImplementingStageRetriesUnexpectedEOF(t *testing.T) {
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 8, 0, time.UTC)
	current, err := manager.Create("todo-eof", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-eof",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	runCalls := 0
	restoreCalls := 0
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			return "same-commit", nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-eof", nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil
		},
		RunLLM: func(AgentRunOptions) (AgentRunResult, error) {
			runCalls++
			if runCalls < 3 {
				return AgentRunResult{SessionID: "ses-eof", ExitCode: 1, Error: "stream error: unexpected EOF"}, nil
			}
			return AgentRunResult{SessionID: "ses-eof-ok", ExitCode: 0}, nil
		},
		RestoreWorkspace: func(string, string) error {
			restoreCalls++
			return nil
		},
	}

	result, err := runImplementingStage(manager, current, item, repoPath, repoPath, opts, "")
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if runCalls != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", runCalls)
	}
	if restoreCalls != 0 {
		t.Fatalf("expected no workspace restore for EOF retry, got %d", restoreCalls)
	}
	if result.Changed {
		t.Fatalf("expected no change after retry")
	}
}

func TestRunImplementingStageContextOverflowStaysInImplementingAfterRetry(t *testing.T) {
	// This test verifies that if both attempts hit context overflow, the job
	// stays in the implementing stage with feedback instead of failing.
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 7, 0, time.UTC)
	current, err := manager.Create("todo-overflow-fail", now, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-overflow-fail",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	runCalls := 0
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			return "same-commit", nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-overflow-fail", nil
		},
		RunLLM: func(AgentRunOptions) (AgentRunResult, error) {
			runCalls++
			// Both calls fail with context overflow
			return AgentRunResult{
				SessionID: "ses-overflow-fail",
				ExitCode:  1,
				Error:     "context overflow: max tokens reached",
			}, nil
		},
		Model: "test-model",
	}

	result, err := runImplementingStage(manager, current, item, repoPath, repoPath, opts, "")
	if err != nil {
		t.Fatalf("expected no error (should retry with feedback), got %v", err)
	}
	if runCalls != 2 {
		t.Fatalf("expected agent to run twice, got %d", runCalls)
	}
	// Should stay in implementing stage with feedback
	if result.Job.Stage != StageImplementing {
		t.Fatalf("expected stage %q, got %q", StageImplementing, result.Job.Stage)
	}
	if result.Job.Feedback == "" {
		t.Fatal("expected feedback to be set")
	}
	if !strings.Contains(result.Job.Feedback, "context overflow") && !strings.Contains(result.Job.Feedback, "Context overflow") {
		t.Fatalf("expected feedback to mention context overflow, got %q", result.Job.Feedback)
	}
	// Changed should be true so we don't skip to project review
	if !result.Changed {
		t.Fatal("expected changed to be true")
	}
}
