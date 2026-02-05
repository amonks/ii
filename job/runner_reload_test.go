package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/todo"
)

func TestReloadTodoReturnsUpdatedTodo(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Original title", todo.CreateOptions{
		Priority:    todo.PriorityPtr(todo.PriorityMedium),
		Description: "Original description",
	})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	// Reload the todo - should see original content
	reloaded, err := reloadTodo(repoPath, created.ID)
	if err != nil {
		t.Fatalf("reload todo: %v", err)
	}
	if reloaded.Title != "Original title" {
		t.Fatalf("expected title %q, got %q", "Original title", reloaded.Title)
	}
	if reloaded.Description != "Original description" {
		t.Fatalf("expected description %q, got %q", "Original description", reloaded.Description)
	}

	// Update the todo
	store, err = todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		t.Fatalf("reopen todo store: %v", err)
	}
	newTitle := "Updated title"
	newDesc := "Updated description"
	_, err = store.Update([]string{created.ID}, todo.UpdateOptions{Title: &newTitle, Description: &newDesc})
	if err != nil {
		store.Release()
		t.Fatalf("update todo: %v", err)
	}
	store.Release()

	// Reload again - should see updated content
	reloaded, err = reloadTodo(repoPath, created.ID)
	if err != nil {
		t.Fatalf("reload todo after update: %v", err)
	}
	if reloaded.Title != "Updated title" {
		t.Fatalf("expected updated title %q, got %q", "Updated title", reloaded.Title)
	}
	if reloaded.Description != "Updated description" {
		t.Fatalf("expected updated description %q, got %q", "Updated description", reloaded.Description)
	}
}

func TestReloadTodoReturnsErrorForMissingTodo(t *testing.T) {
	repoPath := setupJobRepo(t)

	// Create the todo store but don't create any todos
	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	store.Release()

	_, err = reloadTodo(repoPath, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for missing todo")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got %v", err)
	}
}

func TestReloadTodoReturnsErrorForMissingStore(t *testing.T) {
	// Use a temp directory that doesn't have a todo store
	repoPath := t.TempDir()

	_, err := reloadTodo(repoPath, "any-id")
	if err == nil {
		t.Fatal("expected error for missing store")
	}
}

func TestRunReloadsTodoBetweenImplementationRuns(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Original title", todo.CreateOptions{
		Priority:    todo.PriorityPtr(todo.PriorityMedium),
		Description: "Original description",
	})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	llmCount := 0
	observedTitles := []string{}

	// Track commit IDs to properly simulate change detection
	// The pattern is: before1 -> after1 (changed) -> test fails -> before2 -> after2 (changed) -> ...
	commitIDSequence := []string{
		"before-1", "after-1", // First impl: before, after (change detected)
		"before-2", "after-2", // Second impl: before, after (change detected)
		"before-3", "before-3", // Third impl (project review): no change
	}
	commitIdx := 0

	result, err := Run(repoPath, created.ID, RunOptions{
		Now: func() time.Time { return now },
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			// First test run fails to trigger another implementation run
			if llmCount == 1 {
				return []TestCommandResult{{Command: "test", ExitCode: 1, Output: "fail"}}, nil
			}
			return []TestCommandResult{{Command: "test", ExitCode: 0}}, nil
		},
		UpdateStale: func(string) error { return nil },
		RunLLM: func(opts AgentRunOptions) (AgentRunResult, error) {
			llmCount++

			// Extract the title from the prompt to observe what the implementation saw
			// The prompt contains the todo title in a TodoBlock
			if strings.Contains(opts.Prompt, "Original title") {
				observedTitles = append(observedTitles, "Original title")
			} else if strings.Contains(opts.Prompt, "Updated title") {
				observedTitles = append(observedTitles, "Updated title")
			}

			if llmCount == 1 {
				// First implementation run: update the todo for the next run
				store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
				if err != nil {
					return AgentRunResult{}, fmt.Errorf("open store in callback: %w", err)
				}
				newTitle := "Updated title"
				_, err = store.Update([]string{created.ID}, todo.UpdateOptions{Title: &newTitle})
				if err != nil {
					store.Release()
					return AgentRunResult{}, fmt.Errorf("update todo in callback: %w", err)
				}
				store.Release()

				// Write a commit message to indicate changes were made
				messagePath := filepath.Join(opts.WorkspacePath, commitMessageFilename)
				if err := os.WriteFile(messagePath, []byte("feat: first change"), 0o644); err != nil {
					return AgentRunResult{}, err
				}

				return AgentRunResult{SessionID: "session-1", ExitCode: 0}, nil
			}

			// Second implementation run (after test failure)
			if llmCount == 2 {
				// Write a commit message
				messagePath := filepath.Join(opts.WorkspacePath, commitMessageFilename)
				if err := os.WriteFile(messagePath, []byte("feat: second change"), 0o644); err != nil {
					return AgentRunResult{}, err
				}
				return AgentRunResult{SessionID: "session-2", ExitCode: 0}, nil
			}

			// Step review after second implementation
			if llmCount == 3 {
				feedbackPath := filepath.Join(opts.WorkspacePath, feedbackFilename)
				if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
					return AgentRunResult{}, err
				}
				return AgentRunResult{SessionID: "session-3", ExitCode: 0}, nil
			}

			// Third implementation (no changes) then project review
			if llmCount == 4 {
				// No changes, no commit message needed
				return AgentRunResult{SessionID: "session-4", ExitCode: 0}, nil
			}

			// Project review
			feedbackPath := filepath.Join(opts.WorkspacePath, feedbackFilename)
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: fmt.Sprintf("session-%d", llmCount), ExitCode: 0}, nil
		},
		CurrentCommitID: func(string) (string, error) {
			if commitIdx >= len(commitIDSequence) {
				return "same", nil
			}
			id := commitIDSequence[commitIdx]
			commitIdx++
			return id, nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-1", nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			// First two implementations make changes, third does not
			// The check happens after implementation runs 1, 2, 4 (not 3 which is review)
			// At the time of the fourth llmCount call, we want @ to be empty
			if llmCount >= 4 {
				return true, nil // @ is empty for third implementation (project review path)
			}
			return false, nil // @ has changes from implementations 1 and 2
		},
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n1 file changed", nil
		},
		CommitIDAt: func(string, string) (string, error) {
			return "committed", nil
		},
		Commit: func(string, string) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}

	if result.Job.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %q", result.Job.Status)
	}

	// Verify we had at least 2 implementation runs that observed titles
	if len(observedTitles) < 2 {
		t.Fatalf("expected at least 2 implementation runs observing titles, got %d: %v", len(observedTitles), observedTitles)
	}

	// Verify the first implementation saw the original title
	if observedTitles[0] != "Original title" {
		t.Fatalf("expected first implementation to see 'Original title', got %q", observedTitles[0])
	}

	// Verify the second implementation saw the updated title (the todo was
	// updated during the first implementation run, and reloaded before the second)
	if observedTitles[1] != "Updated title" {
		t.Fatalf("expected second implementation to see 'Updated title', got %q", observedTitles[1])
	}
}

func TestRunContextReloadTodoUpdatesItem(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Original", todo.CreateOptions{Priority: todo.PriorityPtr(todo.PriorityLow)})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	var capturedItem todo.Todo
	llmCount := 0

	// Track commit IDs: impl before/after, then same for no-change loop
	commitIDSequence := []string{
		"before-1", "after-1", // First impl: change detected
		"same", "same", // Second impl: no change (triggers project review)
	}
	commitIdx := 0

	// Run a job that updates the todo mid-run and captures the item seen
	// by the review stage
	_, err = Run(repoPath, created.ID, RunOptions{
		Now: func() time.Time { return now },
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			return []TestCommandResult{{Command: "test", ExitCode: 0}}, nil
		},
		UpdateStale: func(string) error { return nil },
		RunLLM: func(opts AgentRunOptions) (AgentRunResult, error) {
			llmCount++

			if llmCount == 1 {
				// Implementation: update the todo DURING the LLM run
				store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
				if err != nil {
					return AgentRunResult{}, err
				}
				newTitle := "Updated"
				_, err = store.Update([]string{created.ID}, todo.UpdateOptions{Title: &newTitle})
				store.Release()
				if err != nil {
					return AgentRunResult{}, err
				}

				// Make changes
				messagePath := filepath.Join(opts.WorkspacePath, commitMessageFilename)
				if err := os.WriteFile(messagePath, []byte("feat: change"), 0o644); err != nil {
					return AgentRunResult{}, err
				}
				return AgentRunResult{SessionID: "impl-1", ExitCode: 0}, nil
			}

			// Step review: capture what title is in the prompt
			// The review prompt still uses the item loaded at the start of the
			// implementing stage (before we updated it during the LLM callback)
			if llmCount == 2 {
				if strings.Contains(opts.Prompt, "Original") {
					capturedItem.Title = "Original"
				} else if strings.Contains(opts.Prompt, "Updated") {
					capturedItem.Title = "Updated"
				}

				feedbackPath := filepath.Join(opts.WorkspacePath, feedbackFilename)
				if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
					return AgentRunResult{}, err
				}
				return AgentRunResult{SessionID: "review-1", ExitCode: 0}, nil
			}

			// Second implementation (no changes) then project review
			if llmCount == 3 {
				return AgentRunResult{SessionID: "impl-2", ExitCode: 0}, nil
			}

			// Project review
			feedbackPath := filepath.Join(opts.WorkspacePath, feedbackFilename)
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
				return AgentRunResult{}, err
			}
			return AgentRunResult{SessionID: fmt.Sprintf("review-%d", llmCount), ExitCode: 0}, nil
		},
		CurrentCommitID: func(string) (string, error) {
			if commitIdx >= len(commitIDSequence) {
				return "same", nil
			}
			id := commitIDSequence[commitIdx]
			commitIdx++
			return id, nil
		},
		CurrentChangeID: func(string) (string, error) {
			return "change-1", nil
		},
		CurrentChangeEmpty: func(string) (bool, error) {
			// First implementation makes changes, second does not
			// At the time of the third llmCount call (impl-2), we want @ to be empty
			if llmCount >= 3 {
				return true, nil // @ is empty for second implementation (project review path)
			}
			return false, nil // @ has changes from first implementation
		},
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n1 file changed", nil
		},
		CommitIDAt: func(string, string) (string, error) {
			return "committed", nil
		},
		Commit: func(string, string) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}

	// The step review stage should see the ORIGINAL title because the todo was
	// loaded at the start of the implementing stage (before we updated it
	// during the LLM callback). This is the expected behavior per the spec:
	// "The re-read todo is used for the remainder of the implement→test→review→commit cycle."
	if capturedItem.Title != "Original" {
		t.Fatalf("expected review to see original title (loaded at impl start), got %q", capturedItem.Title)
	}
}
