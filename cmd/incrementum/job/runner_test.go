package job

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"monks.co/incrementum/internal/config"
	"monks.co/incrementum/todo"
)

func TestTestingStageOutcomeFailure(t *testing.T) {
	results := []TestCommandResult{
		{Command: "go test ./...", ExitCode: 1},
		{Command: "golangci-lint run", ExitCode: 0},
	}

	stage, feedback := testingStageOutcome(results)

	if stage != StageImplementing {
		t.Fatalf("expected stage %q, got %q", StageImplementing, stage)
	}

	expected := FormatTestFeedback(results)
	if feedback != expected {
		t.Fatalf("expected feedback %q, got %q", expected, feedback)
	}
}

func TestTestingStageOutcomeSuccess(t *testing.T) {
	results := []TestCommandResult{{Command: "go test ./...", ExitCode: 0}}

	stage, feedback := testingStageOutcome(results)

	if stage != StageReviewing {
		t.Fatalf("expected stage %q, got %q", StageReviewing, stage)
	}
	if feedback != "" {
		t.Fatalf("expected empty feedback, got %q", feedback)
	}
}

func TestRunTestingStageRequiresTestCommands(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 11, 0, 0, 0, time.UTC)
	current, err := manager.Create("todo-test-missing-tests", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	runCalled := false
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{}, nil
		},
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			runCalled = true
			return nil, fmt.Errorf("unexpected RunTests call")
		},
	}

	_, err = runTestingStage(manager, current, repoPath, workspacePath, opts)
	if err == nil {
		t.Fatal("expected error for missing test commands")
	}
	if !strings.Contains(err.Error(), "test-commands") {
		t.Fatalf("expected test-commands error, got %v", err)
	}
	if runCalled {
		t.Fatal("expected RunTests not to be called")
	}
}

func TestRunImplementingStageReadsCommitMessage(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := "/Users/test/repo"
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 11, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-789", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-789",
		Title:       "Commit message",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitIDs := []string{"before", "after"}
	commitIndex := 0

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
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
			return "change-789", nil
		},
		ChangeIDAt: func(string, string) (string, error) {
			return "change-789", nil
		},
		ChangeIDsForRevset: func(string, string) ([]string, error) {
			return []string{"change-789"}, nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return false, nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			messagePath := filepath.Join(runOpts.WorkspacePath, commitMessageFilename)
			if err := os.WriteFile(messagePath, []byte("feat: step"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: "oc-789", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, created, item, repoPath, workspacePath, opts, "")
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}
	if !result.Changed {
		t.Fatalf("expected change detected")
	}
	if result.CommitMessage != "feat: step" {
		t.Fatalf("expected commit message %q, got %q", "feat: step", result.CommitMessage)
	}
	if result.Job.Stage != StageTesting {
		t.Fatalf("expected stage %q, got %q", StageTesting, result.Job.Stage)
	}
}

func TestRunImplementingStageNoChangesSkipsTesting(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := "/Users/test/repo"
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 11, 5, 0, 0, time.UTC)
	created, err := manager.Create("todo-790", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-790",
		Title:       "No changes",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	messagePath := filepath.Join(workspacePath, commitMessageFilename)
	if err := os.WriteFile(messagePath, []byte("old message"), 0o644); err != nil {
		t.Fatalf("seed commit message: %v", err)
	}

	commitIDs := []string{"same", "same"}
	commitIndex := 0

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
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
			return "change-790", nil
		},
		ChangeIDAt: func(string, string) (string, error) {
			return "change-790", nil
		},
		ChangeIDsForRevset: func(string, string) ([]string, error) {
			return []string{"change-790"}, nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil // @ is empty (no uncommitted work)
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			return AgentRunResult{SessionID: "oc-790", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, created, item, repoPath, workspacePath, opts, "")
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}
	if result.Changed {
		t.Fatalf("expected no change detected")
	}
	if result.CommitMessage != "" {
		t.Fatalf("expected empty commit message, got %q", result.CommitMessage)
	}
	if result.Job.Stage != StageReviewing {
		t.Fatalf("expected stage %q, got %q", StageReviewing, result.Job.Stage)
	}
	if _, err := os.Stat(messagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected commit message removed, got %v", err)
	}
}

func TestRunImplementingStageIncludesCommitMessageInstructionWithFeedback(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 11, 10, 0, 0, time.UTC)
	created, err := manager.Create("todo-111", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	created.Feedback = "Tests failed"

	item := todo.Todo{
		ID:          "todo-111",
		Title:       "Retry with feedback",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	previousMessage := "feat: earlier draft"

	commitCalls := 0
	var seenPrompt string
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		CurrentCommitID: func(string) (string, error) {
			commitCalls++
			if commitCalls > 2 {
				return "", fmt.Errorf("commit id lookup exhausted")
			}
			return "same", nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-111", nil
		},
		ChangeIDAt: func(string, string) (string, error) {
			return "change-111", nil
		},
		ChangeIDsForRevset: func(string, string) ([]string, error) {
			return []string{"change-111"}, nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return true, nil // @ is empty after feedback run
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			seenPrompt = runOpts.Prompt.UserContent
			return AgentRunResult{SessionID: "oc-111", ExitCode: 0}, nil
		},
	}

	_, err = runImplementingStage(manager, created, item, repoPath, workspacePath, opts, previousMessage)
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}

	if !strings.Contains(seenPrompt, "Previous feedback") {
		t.Fatalf("expected prompt to include feedback, got %q", seenPrompt)
	}
}

func TestRunReviewingStagePassesCommitMessage(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := "/Users/test/repo"
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 11, 30, 0, 0, time.UTC)
	created, err := manager.Create("todo-456", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-456",
		Title:       "Review commit",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitMessage := "feat: add review message"
	var seenPrompt string
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			seenPrompt = runOpts.Prompt.UserContent
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: "oc-456", ExitCode: 0}, nil
		},
		Config: &config.Config{Job: config.Job{TestCommands: []string{"go test ./..."}}},
	}

	result, err := runReviewingStage(manager, created, item, repoPath, workspacePath, opts, commitMessage, reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}

	if !strings.Contains(seenPrompt, commitMessage) {
		t.Fatalf("expected prompt to include commit message, got %q", seenPrompt)
	}
	if result.Job.Stage != StageCommitting {
		t.Fatalf("expected stage %q, got %q", StageCommitting, result.Job.Stage)
	}
}

func TestRunReviewingStageReadsCommitMessageFile(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 30, 0, 0, time.UTC)
	created, err := manager.Create("todo-987", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-987",
		Title:       "Review commit file",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitMessage := "fix: include review prompt"
	messagePath := filepath.Join(workspacePath, commitMessageFilename)
	if err := os.WriteFile(messagePath, []byte(commitMessage), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	var seenPrompt string
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			seenPrompt = runOpts.Prompt.UserContent
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: "oc-789", ExitCode: 0}, nil
		},
		Config: &config.Config{Job: config.Job{TestCommands: []string{"go test ./..."}}},
	}

	_, err = runReviewingStage(manager, created, item, repoPath, workspacePath, opts, "", reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}

	if !strings.Contains(seenPrompt, commitMessage) {
		t.Fatalf("expected prompt to include commit message, got %q", seenPrompt)
	}
	if _, err := os.Stat(messagePath); !os.IsNotExist(err) {
		t.Fatalf("expected commit message file to be deleted")
	}
}

func TestRunReviewingStageMissingCommitMessageExplainsContext(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 40, 0, 0, time.UTC)
	current, err := manager.Create("todo-123", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-123",
		Title:       "Review commit missing message",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	calledLLM := false
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			calledLLM = true
			return AgentRunResult{SessionID: "oc-123", ExitCode: 0}, nil
		},
		Config: &config.Config{Job: config.Job{TestCommands: []string{"go test ./..."}}},
	}

	_, err = runReviewingStage(manager, current, item, repoPath, workspacePath, opts, "", reviewScopeStep)
	if err == nil {
		t.Fatal("expected missing commit message error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing file error, got %v", err)
	}
	if calledLLM {
		t.Fatalf("expected review to stop before LLM")
	}
	if !strings.Contains(err.Error(), "commit message missing before LLM review") {
		t.Fatalf("expected context in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "LLM implementation") {
		t.Fatalf("expected author context, got %v", err)
	}
	if !strings.Contains(err.Error(), commitMessageFilename) {
		t.Fatalf("expected commit message path context, got %v", err)
	}
}

func TestRunReviewingStageInjectsCommitMessageWhenTemplateMissing(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	promptDir := filepath.Join(workspacePath, ".incrementum", "templates")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}
	customPrompt := "Review the changes in the jujutsu working tree."
	if err := os.WriteFile(filepath.Join(promptDir, "prompt-commit-review.tmpl"), []byte(customPrompt), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 45, 0, 0, time.UTC)
	created, err := manager.Create("todo-654", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-654",
		Title:       "Review commit fallback",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitMessage := "fix: inject review prompt"
	var seenPrompt string
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			seenPrompt = runOpts.Prompt.UserContent
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: "oc-654", ExitCode: 0}, nil
		},
		Config: &config.Config{Job: config.Job{TestCommands: []string{"go test ./..."}}},
	}

	_, err = runReviewingStage(manager, created, item, repoPath, workspacePath, opts, commitMessage, reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}

	if !strings.Contains(seenPrompt, "Change description\n\n    "+commitMessage) {
		t.Fatalf("expected prompt to include injected change description block, got %q", seenPrompt)
	}
	if !strings.Contains(seenPrompt, commitMessage) {
		t.Fatalf("expected prompt to include change description, got %q", seenPrompt)
	}
}

func TestRunCommittingStageFormatsCommitMessage(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 13, 0, 0, 0, time.UTC)
	current, err := manager.Create("todo-333", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	appendSession := AgentSession{Purpose: "implement", ID: "ses-333"}
	current, err = manager.Update(current.ID, UpdateOptions{AppendAgentSession: &appendSession}, startedAt)
	if err != nil {
		t.Fatalf("append agent session: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-333",
		Title:       "Expand commit message",
		Description: "Add todo metadata and transcripts.",
		Status:      todo.StatusOpen,
		Type:        todo.TypeTask,
		Priority:    todo.PriorityHigh,
	}

	var captured string
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n", nil
		},
		Transcripts: func(repoPath string, sessions []AgentSession) ([]AgentTranscript, error) {
			return []AgentTranscript{{Purpose: "implement", Transcript: "Planning\n"}}, nil
		},
		CommitIDAt: func(string, string) (string, error) {
			return "commit-333", nil
		},
	}
	opts.Commit = func(string, message string) error {
		captured = message
		return nil
	}

	_, err = runCommittingStage(CommittingStageOptions{
		Manager:       manager,
		Current:       current,
		Item:          item,
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		RunOptions:    opts,
		Result:        &RunResult{},
		CommitMessage: "feat: expand commit metadata",
	})
	if err != nil {
		t.Fatalf("run committing stage: %v", err)
	}

	checks := []string{
		"feat: expand commit metadata",
		"Here is a generated change description:",
		"This change is part of a series implementing this todo:",
		"    ID: todo-333",
		"    Title: Expand commit message",
		"    Type: task",
		"    Priority: 1 (high)",
		"    Description:",
		"        Add todo metadata and transcripts.",
	}
	for _, check := range checks {
		if !strings.Contains(captured, check) {
			t.Fatalf("expected commit message to include %q, got %q", check, captured)
		}
	}
}

func TestRunCommittingStageLogsFormattedCommitMessage(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()
	eventsDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 13, 5, 0, 0, time.UTC)
	current, err := manager.Create("todo-commit-log", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-commit-log",
		Title:       "Log commit message",
		Description: "Ensure final commit message logs use the log width.",
		Status:      todo.StatusOpen,
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	log, err := OpenEventLog(current.ID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("close event log: %v", err)
		}
	}()

	message := "feat: log commit message"
	expectedLogMessage := formatCommitMessageWithWidth(item, message, "", lineWidth-subdocumentIndent)

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n", nil
		},
		CommitIDAt: func(string, string) (string, error) {
			return "commit-log", nil
		},
		Commit: func(string, string) error {
			return nil
		},
		EventLog: log,
	}

	_, err = runCommittingStage(CommittingStageOptions{
		Manager:       manager,
		Current:       current,
		Item:          item,
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		RunOptions:    opts,
		Result:        &RunResult{},
		CommitMessage: message,
	})
	if err != nil {
		t.Fatalf("run committing stage: %v", err)
	}

	path := filepath.Join(eventsDir, current.ID+".jsonl")
	events := readEventLogFile(t, path)
	if len(events) == 0 {
		t.Fatal("expected event log entries")
	}

	var commitEvent commitMessageEventData
	for _, event := range events {
		if event.Name != jobEventCommitMessage {
			continue
		}
		if err := json.Unmarshal([]byte(event.Data), &commitEvent); err != nil {
			t.Fatalf("decode commit message event: %v", err)
		}
		break
	}
	if commitEvent.Message == "" {
		t.Fatalf("expected commit message event, got %v", events)
	}
	if commitEvent.Message != expectedLogMessage {
		t.Fatalf("expected log message %q, got %q", expectedLogMessage, commitEvent.Message)
	}
	if !commitEvent.Preformatted {
		t.Fatalf("expected preformatted commit message, got %#v", commitEvent)
	}
}

func TestRunCommittingStageSkipsEmptyChange(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 13, 7, 0, 0, time.UTC)
	current, err := manager.Create("todo-empty-change", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-empty-change",
		Title:    "Skip empty change",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	commitCalls := 0
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "0 files changed, 0 insertions(+), 0 deletions(-)\n", nil
		},
		Commit: func(string, string) error {
			commitCalls++
			return nil
		},
	}

	updated, err := runCommittingStage(CommittingStageOptions{
		Manager:       manager,
		Current:       current,
		Item:          item,
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		RunOptions:    opts,
		Result:        &RunResult{},
		CommitMessage: "feat: nothing to commit",
	})
	if err != nil {
		t.Fatalf("run committing stage: %v", err)
	}
	if commitCalls != 0 {
		t.Fatalf("expected no commit attempt, got %d", commitCalls)
	}
	if updated.Stage != StageImplementing {
		t.Fatalf("expected stage %q, got %q", StageImplementing, updated.Stage)
	}
}

func TestDiffStatHasChangesDetectsEmptySummaries(t *testing.T) {
	cases := []struct {
		name     string
		diffStat string
		changed  bool
	}{
		{
			name:     "empty output",
			diffStat: "\n\n",
			changed:  false,
		},
		{
			name:     "no changes line",
			diffStat: "No changes.\n",
			changed:  false,
		},
		{
			name:     "zero summary after header",
			diffStat: "Working copy is clean\n0 files changed, 0 insertions(+), 0 deletions(-)\n",
			changed:  false,
		},
		{
			name:     "header without file stats",
			diffStat: "Working copy changes:\n\n",
			changed:  false,
		},
		{
			name:     "summary with changes",
			diffStat: "2 files changed, 3 insertions(+), 1 deletion(-)\n",
			changed:  true,
		},
		{
			name:     "file changes",
			diffStat: "file.txt | 2 +-\n1 file changed, 1 insertion(+), 1 deletion(-)\n",
			changed:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := diffStatHasChanges(tc.diffStat); got != tc.changed {
				t.Fatalf("expected changed=%t, got %t", tc.changed, got)
			}
		})
	}
}

func TestRunImplementingStageCreatesJobChange(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := "/Users/test/repo"
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-change-track", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-change-track",
		Title:       "Track changes",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitIDs := []string{"before", "after"}
	commitIndex := 0

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
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
			return "change-abc123", nil
		},
		ChangeIDAt: func(string, string) (string, error) {
			return "change-abc123", nil
		},
		ChangeIDsForRevset: func(string, string) ([]string, error) {
			return []string{"change-abc123"}, nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			return false, nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			messagePath := filepath.Join(runOpts.WorkspacePath, commitMessageFilename)
			if err := os.WriteFile(messagePath, []byte("feat: track changes"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: "oc-change-track", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, created, item, repoPath, workspacePath, opts, "")
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}

	// Verify the job has a change created
	if len(result.Job.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Job.Changes))
	}
	change := result.Job.Changes[0]
	if change.ChangeID != "change-abc123" {
		t.Fatalf("expected change id %q, got %q", "change-abc123", change.ChangeID)
	}

	// Verify the change has a commit
	if len(change.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(change.Commits))
	}
	commit := change.Commits[0]
	if commit.CommitID != "after" {
		t.Fatalf("expected commit id %q, got %q", "after", commit.CommitID)
	}
	if commit.DraftMessage != "feat: track changes" {
		t.Fatalf("expected draft message %q, got %q", "feat: track changes", commit.DraftMessage)
	}
	if commit.AgentSessionID != "oc-change-track" {
		t.Fatalf("expected agent session id %q, got %q", "oc-change-track", commit.AgentSessionID)
	}
}

func TestRunTestingStageUpdatesCommitTestsPassed(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 20, 11, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-test-pass", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Add a change with a commit first
	created, err = manager.AppendChange(created.ID, JobChange{ChangeID: "change-test"}, startedAt)
	if err != nil {
		t.Fatalf("append change: %v", err)
	}
	created, err = manager.AppendCommitToCurrentChange(created.ID, JobCommit{
		CommitID:       "commit-test",
		DraftMessage:   "feat: test",
		AgentSessionID: "ses-test",
	}, startedAt)
	if err != nil {
		t.Fatalf("append commit: %v", err)
	}

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{
				Job: config.Job{
					TestCommands: []string{"echo ok"},
				},
			}, nil
		},
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			return []TestCommandResult{{Command: "echo ok", ExitCode: 0}}, nil
		},
	}

	result, err := runTestingStage(manager, created, repoPath, workspacePath, opts)
	if err != nil {
		t.Fatalf("run testing stage: %v", err)
	}

	// Verify the commit has tests_passed set to true
	if len(result.Changes) == 0 || len(result.Changes[0].Commits) == 0 {
		t.Fatalf("expected change with commit, got %v", result.Changes)
	}
	commit := result.Changes[0].Commits[0]
	if commit.TestsPassed == nil {
		t.Fatalf("expected tests passed to be set")
	}
	if *commit.TestsPassed != true {
		t.Fatalf("expected tests passed true, got %v", *commit.TestsPassed)
	}
}

func TestRunTestingStageUpdatesCommitTestsFailed(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 20, 11, 30, 0, 0, time.UTC)
	created, err := manager.Create("todo-test-fail", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Add a change with a commit first
	created, err = manager.AppendChange(created.ID, JobChange{ChangeID: "change-fail"}, startedAt)
	if err != nil {
		t.Fatalf("append change: %v", err)
	}
	created, err = manager.AppendCommitToCurrentChange(created.ID, JobCommit{
		CommitID:       "commit-fail",
		DraftMessage:   "feat: test fail",
		AgentSessionID: "ses-fail",
	}, startedAt)
	if err != nil {
		t.Fatalf("append commit: %v", err)
	}

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{
				Job: config.Job{
					TestCommands: []string{"go test ./..."},
				},
			}, nil
		},
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			return []TestCommandResult{{Command: "go test ./...", ExitCode: 1, Output: "FAIL"}}, nil
		},
	}

	result, err := runTestingStage(manager, created, repoPath, workspacePath, opts)
	if err != nil {
		t.Fatalf("run testing stage: %v", err)
	}

	// Verify the commit has tests_passed set to false
	if len(result.Changes) == 0 || len(result.Changes[0].Commits) == 0 {
		t.Fatalf("expected change with commit, got %v", result.Changes)
	}
	commit := result.Changes[0].Commits[0]
	if commit.TestsPassed == nil {
		t.Fatalf("expected tests passed to be set")
	}
	if *commit.TestsPassed != false {
		t.Fatalf("expected tests passed false, got %v", *commit.TestsPassed)
	}
}

func TestRunReviewingStageUpdatesCommitReview(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := "/Users/test/repo"
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 20, 12, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-review", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Add a change with a commit first
	created, err = manager.AppendChange(created.ID, JobChange{ChangeID: "change-review"}, startedAt)
	if err != nil {
		t.Fatalf("append change: %v", err)
	}
	created, err = manager.AppendCommitToCurrentChange(created.ID, JobCommit{
		CommitID:       "commit-review",
		DraftMessage:   "feat: review",
		AgentSessionID: "ses-review-impl",
	}, startedAt)
	if err != nil {
		t.Fatalf("append commit: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-review",
		Title:       "Review commit",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n\nlooks good"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: "oc-review", ExitCode: 0}, nil
		},
		Config: &config.Config{Job: config.Job{TestCommands: []string{"go test ./..."}}},
	}

	commitMessage := "feat: review"
	result, err := runReviewingStage(manager, created, item, repoPath, workspacePath, opts, commitMessage, reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}

	// Verify the commit has review set
	if len(result.Job.Changes) == 0 || len(result.Job.Changes[0].Commits) == 0 {
		t.Fatalf("expected change with commit, got %v", result.Job.Changes)
	}
	commit := result.Job.Changes[0].Commits[0]
	if commit.Review == nil {
		t.Fatalf("expected review to be set")
	}
	if commit.Review.Outcome != ReviewOutcomeAccept {
		t.Fatalf("expected review outcome %q, got %q", ReviewOutcomeAccept, commit.Review.Outcome)
	}
	if commit.Review.Comments != "looks good" {
		t.Fatalf("expected review comments %q, got %q", "looks good", commit.Review.Comments)
	}
	if commit.Review.AgentSessionID != "oc-review" {
		t.Fatalf("expected review session id %q, got %q", "oc-review", commit.Review.AgentSessionID)
	}
}

func TestRunReviewingStageProjectSetsProjectReview(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := "/Users/test/repo"
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 20, 13, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-project-review", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-project-review",
		Title:       "Project review",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		RunLLM: func(runOpts AgentRunOptions) (AgentRunResult, error) {
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n\nproject complete"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: "oc-project-review", ExitCode: 0}, nil
		},
		Config: &config.Config{Job: config.Job{TestCommands: []string{"go test ./..."}}},
	}

	result, err := runReviewingStage(manager, created, item, repoPath, workspacePath, opts, "", reviewScopeProject)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}

	// Verify project review is set
	if result.Job.ProjectReview == nil {
		t.Fatalf("expected project review to be set")
	}
	if result.Job.ProjectReview.Outcome != ReviewOutcomeAccept {
		t.Fatalf("expected project review outcome %q, got %q", ReviewOutcomeAccept, result.Job.ProjectReview.Outcome)
	}
	if result.Job.ProjectReview.Comments != "project complete" {
		t.Fatalf("expected project review comments %q, got %q", "project complete", result.Job.ProjectReview.Comments)
	}
	if result.Job.ProjectReview.AgentSessionID != "oc-project-review" {
		t.Fatalf("expected project review session id %q, got %q", "oc-project-review", result.Job.ProjectReview.AgentSessionID)
	}
}

func TestBuildLLMFailureMessageIncludesExitCodeAndError(t *testing.T) {
	result := AgentRunResult{
		SessionID: "session-123",
		ExitCode:  1,
		Error:     "context deadline exceeded",
	}
	runOpts := AgentRunOptions{
		Model:         "claude-sonnet-4",
		RepoPath:      "/repo",
		WorkspacePath: "/workspace",
	}

	msg := buildLLMFailureMessage("implement", "prompt-implementation.tmpl", result, runOpts, "before-commit", "after-commit", nil, false, nil, 0)

	// Check that message starts with the unified format prefix
	expectedPrefix := "agent implement failed with exit code 1:"
	if !strings.HasPrefix(msg, expectedPrefix) {
		t.Errorf("expected message to start with %q, got %q", expectedPrefix, msg)
	}

	// Check for error reason
	if !strings.Contains(msg, "error: context deadline exceeded") {
		t.Errorf("expected error reason in message, got %q", msg)
	}

	// Check for session ID
	if !strings.Contains(msg, "session session-123") {
		t.Errorf("expected session ID in message, got %q", msg)
	}

	// Check for model
	if !strings.Contains(msg, `model "claude-sonnet-4"`) {
		t.Errorf("expected model in message, got %q", msg)
	}
}

func TestBuildLLMFailureMessageNegativeExitCode(t *testing.T) {
	result := AgentRunResult{
		SessionID: "session-456",
		ExitCode:  -1,
	}
	runOpts := AgentRunOptions{
		Model: "claude-sonnet-4",
	}

	msg := buildLLMFailureMessage("implement", "prompt-implementation.tmpl", result, runOpts, "", "", nil, false, nil, 0)

	// Check that message starts with the unified format prefix including parenthetical
	expectedPrefix := "agent implement failed with exit code -1 (process did not exit cleanly):"
	if !strings.HasPrefix(msg, expectedPrefix) {
		t.Errorf("expected message to start with %q, got %q", expectedPrefix, msg)
	}
}

func TestBuildLLMFailureMessageWithoutError(t *testing.T) {
	result := AgentRunResult{
		SessionID: "session-789",
		ExitCode:  1,
		Error:     "", // No error message available (e.g., external backend)
	}
	runOpts := AgentRunOptions{
		Model: "claude-sonnet-4",
	}

	msg := buildLLMFailureMessage("implement", "prompt-implementation.tmpl", result, runOpts, "", "", nil, false, nil, 0)

	// Check that message starts with the unified format prefix
	expectedPrefix := "agent implement failed with exit code 1:"
	if !strings.HasPrefix(msg, expectedPrefix) {
		t.Errorf("expected message to start with %q, got %q", expectedPrefix, msg)
	}

	// Should NOT contain "error:" prefix when Error is empty
	if strings.Contains(msg, "error:") {
		t.Errorf("did not expect error prefix when Error is empty, got %q", msg)
	}
}

func TestBuildReviewFailureMessageIncludesExitCodeAndError(t *testing.T) {
	result := AgentRunResult{
		SessionID: "session-review-123",
		ExitCode:  1,
		Error:     "API rate limit exceeded",
	}

	msg := buildReviewFailureMessage("review", result, "claude-opus-4")

	// Check that message starts with the unified format prefix
	expectedPrefix := "agent review failed with exit code 1:"
	if !strings.HasPrefix(msg, expectedPrefix) {
		t.Errorf("expected message to start with %q, got %q", expectedPrefix, msg)
	}

	// Check for error reason
	if !strings.Contains(msg, "error: API rate limit exceeded") {
		t.Errorf("expected error reason in message, got %q", msg)
	}

	// Check for session ID
	if !strings.Contains(msg, "session session-review-123") {
		t.Errorf("expected session ID in message, got %q", msg)
	}

	// Check for model
	if !strings.Contains(msg, `model "claude-opus-4"`) {
		t.Errorf("expected model in message, got %q", msg)
	}
}

func TestBuildReviewFailureMessageNegativeExitCode(t *testing.T) {
	result := AgentRunResult{
		SessionID: "session-review-456",
		ExitCode:  -1,
		Error:     "signal: killed",
	}

	msg := buildReviewFailureMessage("project-review", result, "claude-sonnet-4")

	// Check that message starts with the unified format prefix including parenthetical
	expectedPrefix := "agent project-review failed with exit code -1 (process did not exit cleanly):"
	if !strings.HasPrefix(msg, expectedPrefix) {
		t.Errorf("expected message to start with %q, got %q", expectedPrefix, msg)
	}
}

func TestBuildReviewFailureMessageWithoutError(t *testing.T) {
	result := AgentRunResult{
		SessionID: "session-review-789",
		ExitCode:  1,
		Error:     "", // No error message available (e.g., external backend)
	}

	msg := buildReviewFailureMessage("review", result, "claude-sonnet-4")

	// Check that message starts with the unified format prefix
	expectedPrefix := "agent review failed with exit code 1:"
	if !strings.HasPrefix(msg, expectedPrefix) {
		t.Errorf("expected message to start with %q, got %q", expectedPrefix, msg)
	}

	// Should NOT contain "error:" prefix when Error is empty
	if strings.Contains(msg, "error:") {
		t.Errorf("did not expect error prefix when Error is empty, got %q", msg)
	}

	// Should still include session and model
	if !strings.Contains(msg, "session session-review-789") {
		t.Errorf("expected session ID in message, got %q", msg)
	}
}
