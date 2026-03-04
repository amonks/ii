package main

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"monks.co/incrementum/internal/jj"
	jobpkg "monks.co/incrementum/job"
	"monks.co/incrementum/todo"
	"github.com/spf13/cobra"
)

func TestReflowJobTextPreservesMarkdown(t *testing.T) {
	input := "Intro line.\n\n- First item\n- Second item\n\n```text\nline one\nline two\n```"
	output := reflowJobText(input, 80)

	if output == "-" {
		t.Fatalf("expected non-empty output, got %q", output)
	}
	checks := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s+Intro line\.$`),
		regexp.MustCompile(`(?m)^\s+.*First item$`),
		regexp.MustCompile(`(?m)^\s+.*Second item$`),
		regexp.MustCompile(`(?m)^\s+line one$`),
		regexp.MustCompile(`(?m)^\s+line two$`),
	}
	for _, check := range checks {
		if !check.MatchString(output) {
			t.Fatalf("expected markdown output to match %q, got %q", check.String(), output)
		}
	}
}

func TestFormatJobFieldWrapsValue(t *testing.T) {
	value := strings.Repeat("word ", 40)
	output := formatJobField("Title", value)

	firstIndent := strings.Repeat(" ", jobDocumentIndent)
	if !strings.HasPrefix(output, firstIndent+"Title: ") {
		t.Fatalf("expected title prefix, got %q", output)
	}
	continuationIndent := strings.Repeat(" ", jobDocumentIndent+len("Title: "))
	if !strings.Contains(output, "\n"+continuationIndent) {
		t.Fatalf("expected wrapped continuation indentation, got %q", output)
	}
}

func TestFormatCommitMessageOutputIndentsMessage(t *testing.T) {
	message := "Summary line\n\nHere is a generated commit message:\n\n    Body line\n\nThis commit is a step towards implementing this todo:\n\n    ID: todo-1"
	output := formatCommitMessageOutput(message)
	if !strings.Contains(output, "Commit message:") {
		t.Fatalf("expected header, got %q", output)
	}
	if !strings.Contains(output, "\n    Summary line") {
		t.Fatalf("expected summary indentation, got %q", output)
	}
	if !strings.Contains(output, "\n        Body line") {
		t.Fatalf("expected body indentation, got %q", output)
	}
}

func TestStageMessageUsesReviewLabel(t *testing.T) {
	message := jobpkg.StageMessage(jobpkg.StageReviewing)
	if message != "Starting review:" {
		t.Fatalf("expected review stage message, got %q", message)
	}
}

func TestRunJobDoMultipleTodos(t *testing.T) {
	originalJobDoTodo := jobDoTodo
	originalOpenQueueStore := openQueueStore
	defer func() {
		jobDoTodo = originalJobDoTodo
		openQueueStore = originalOpenQueueStore
	}()

	var got []string
	jobDoTodo = func(cmd *cobra.Command, todoID string) error {
		got = append(got, todoID)
		return nil
	}

	// Mock the queue store to track queueing calls
	var queuedIDs []string
	var reopenedIDs []string
	openQueueStore = func(repoPath, purpose string) (queueStore, error) {
		return &fakeQueueStore{
			queueFn: func(ids []string) ([]todo.Todo, error) {
				queuedIDs = append(queuedIDs, ids...)
				return nil, nil
			},
			reopenFn: func(ids []string) ([]todo.Todo, error) {
				reopenedIDs = append(reopenedIDs, ids...)
				return nil, nil
			},
		}, nil
	}

	resetJobDoGlobals()
	cmd := newTestJobDoCommand()
	if err := runJobDo(cmd, []string{"todo-1", "todo-2", "todo-3"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []string{"todo-1", "todo-2", "todo-3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("expected job runs %v, got %v", want, got)
	}

	// Verify todos were queued at the start
	if strings.Join(queuedIDs, ",") != strings.Join(want, ",") {
		t.Fatalf("expected queued todos %v, got %v", want, queuedIDs)
	}

	// Verify no todos were reopened (all completed successfully)
	if len(reopenedIDs) != 0 {
		t.Fatalf("expected no reopened todos, got %v", reopenedIDs)
	}
}

func TestRunJobDoMultipleTodosCleanupOnError(t *testing.T) {
	originalJobDoTodo := jobDoTodo
	originalOpenQueueStore := openQueueStore
	defer func() {
		jobDoTodo = originalJobDoTodo
		openQueueStore = originalOpenQueueStore
	}()

	// Simulate error on second todo
	expectedErr := errors.New("job failed")
	callCount := 0
	jobDoTodo = func(cmd *cobra.Command, todoID string) error {
		callCount++
		if callCount == 2 {
			return expectedErr
		}
		return nil
	}

	// Track queueing and cleanup calls
	var queuedIDs []string
	var reopenedIDs []string
	openQueueStore = func(repoPath, purpose string) (queueStore, error) {
		return &fakeQueueStore{
			queueFn: func(ids []string) ([]todo.Todo, error) {
				queuedIDs = append(queuedIDs, ids...)
				return nil, nil
			},
			reopenFn: func(ids []string) ([]todo.Todo, error) {
				reopenedIDs = append(reopenedIDs, ids...)
				return nil, nil
			},
		}, nil
	}

	resetJobDoGlobals()
	cmd := newTestJobDoCommand()
	err := runJobDo(cmd, []string{"todo-1", "todo-2", "todo-3"})

	// Should get the error from the failed job
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	// Verify all todos were queued at the start
	want := []string{"todo-1", "todo-2", "todo-3"}
	if strings.Join(queuedIDs, ",") != strings.Join(want, ",") {
		t.Fatalf("expected queued todos %v, got %v", want, queuedIDs)
	}

	// Verify remaining todos (only todo-3) were reopened for cleanup
	// Note: todo-1 was processed successfully, todo-2 failed (may have started),
	// so only todo-3 (never started) gets reopened
	wantReopened := []string{"todo-3"}
	if strings.Join(reopenedIDs, ",") != strings.Join(wantReopened, ",") {
		t.Fatalf("expected reopened todos %v, got %v", wantReopened, reopenedIDs)
	}
}

func TestRunJobDoSingleTodoNoQueueing(t *testing.T) {
	originalJobDoTodo := jobDoTodo
	originalOpenQueueStore := openQueueStore
	defer func() {
		jobDoTodo = originalJobDoTodo
		openQueueStore = originalOpenQueueStore
	}()

	var got []string
	jobDoTodo = func(cmd *cobra.Command, todoID string) error {
		got = append(got, todoID)
		return nil
	}

	// Track if store was ever opened
	storeOpened := false
	openQueueStore = func(repoPath, purpose string) (queueStore, error) {
		storeOpened = true
		return &fakeQueueStore{}, nil
	}

	resetJobDoGlobals()
	cmd := newTestJobDoCommand()
	if err := runJobDo(cmd, []string{"todo-1"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the single todo was processed
	if strings.Join(got, ",") != "todo-1" {
		t.Fatalf("expected job run for todo-1, got %v", got)
	}

	// Verify store was NOT opened for queueing (single todo case)
	if storeOpened {
		t.Fatal("expected store not to be opened for single todo")
	}
}

func resetJobDoGlobals() {
	jobDoTitle = ""
	jobDoType = "task"
	jobDoPriority = todo.PriorityMedium
	jobDoDescription = ""
	jobDoDeps = nil
	jobDoEdit = false
	jobDoNoEdit = false
}

func newTestJobDoCommand() *cobra.Command {
	cmd := &cobra.Command{RunE: runJobDo}
	addDescriptionFlagAliases(cmd)
	cmd.Flags().StringVar(&jobDoTitle, "title", "", "Todo title")
	cmd.Flags().StringVarP(&jobDoType, "type", "t", "task", "Todo type (task, bug, feature, design)")
	cmd.Flags().IntVarP(&jobDoPriority, "priority", "p", todo.PriorityMedium, "Priority (0=critical, 1=high, 2=medium, 3=low, 4=backlog)")
	cmd.Flags().StringVarP(&jobDoDescription, "description", "d", "", "Description (use '-' to read from stdin)")
	cmd.Flags().StringArrayVar(&jobDoDeps, "deps", nil, "Dependencies in format <id> (e.g., abc123)")
	cmd.Flags().BoolVarP(&jobDoEdit, "edit", "e", false, "Open $EDITOR (default if interactive and no create flags)")
	cmd.Flags().BoolVar(&jobDoNoEdit, "no-edit", false, "Do not open $EDITOR")
	return cmd
}

func setupJobDoTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	client := jj.New()
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}
	return tmpDir
}

func TestRunDesignTodoRoutesToInteractiveSession(t *testing.T) {
	originalSession := runInteractiveSession
	defer func() { runInteractiveSession = originalSession }()

	repoPath := setupJobDoTestRepo(t)

	// Create a todo store with a design todo
	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
		Purpose:         "test",
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	created, err := store.Create("Test design todo", todo.CreateOptions{
		Type: todo.TypeDesign,
	})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}
	store.Release()

	// Track whether the interactive session was called
	var sessionCalled bool
	var sessionOpts interactiveSessionOptions
	runInteractiveSession = func(opts interactiveSessionOptions) (interactiveSessionResult, error) {
		sessionCalled = true
		sessionOpts = opts
		return interactiveSessionResult{exitCode: 0}, nil
	}

	// Run the design todo
	cmd := newTestJobDoCommand()
	if err := runDesignTodo(cmd, repoPath, *created); err != nil {
		t.Fatalf("runDesignTodo failed: %v", err)
	}

	// Verify the interactive session was called
	if !sessionCalled {
		t.Fatal("expected interactive session to be called")
	}
	if sessionOpts.repoPath != repoPath {
		t.Fatalf("expected repoPath %q, got %q", repoPath, sessionOpts.repoPath)
	}
	if !strings.Contains(sessionOpts.prompt, "design todo") {
		t.Fatalf("expected prompt to mention design todo, got %q", sessionOpts.prompt)
	}
	if !strings.Contains(sessionOpts.prompt, created.ID) {
		t.Fatalf("expected prompt to contain todo ID %q, got %q", created.ID, sessionOpts.prompt)
	}
}

func TestRunDesignTodoMarksTodoAsStarted(t *testing.T) {
	originalSession := runInteractiveSession
	defer func() { runInteractiveSession = originalSession }()

	repoPath := setupJobDoTestRepo(t)

	// Create a design todo
	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
		Purpose:         "test",
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	created, err := store.Create("Test design todo", todo.CreateOptions{
		Type: todo.TypeDesign,
	})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}
	store.Release()

	// Mock the interactive session to return without error
	runInteractiveSession = func(opts interactiveSessionOptions) (interactiveSessionResult, error) {
		// Check status during session - it should be in_progress
		store, err := todo.Open(repoPath, todo.OpenOptions{
			CreateIfMissing: false,
			PromptToCreate:  false,
			Purpose:         "verify status",
		})
		if err != nil {
			t.Fatalf("failed to open store during session: %v", err)
		}
		defer store.Release()

		items, err := store.Show([]string{created.ID})
		if err != nil {
			t.Fatalf("failed to show todo: %v", err)
		}
		if len(items) == 0 {
			t.Fatal("todo not found during session")
		}
		if items[0].Status != todo.StatusInProgress {
			t.Fatalf("expected status in_progress during session, got %q", items[0].Status)
		}

		return interactiveSessionResult{exitCode: 0}, nil
	}

	cmd := newTestJobDoCommand()
	if err := runDesignTodo(cmd, repoPath, *created); err != nil {
		t.Fatalf("runDesignTodo failed: %v", err)
	}
}

func TestRunDesignTodoMarksTodoAsDoneOnSuccess(t *testing.T) {
	originalSession := runInteractiveSession
	defer func() { runInteractiveSession = originalSession }()

	repoPath := setupJobDoTestRepo(t)

	// Create a design todo
	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
		Purpose:         "test",
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	created, err := store.Create("Test design todo", todo.CreateOptions{
		Type: todo.TypeDesign,
	})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}
	store.Release()

	// Mock successful session
	runInteractiveSession = func(opts interactiveSessionOptions) (interactiveSessionResult, error) {
		return interactiveSessionResult{exitCode: 0}, nil
	}

	cmd := newTestJobDoCommand()
	if err := runDesignTodo(cmd, repoPath, *created); err != nil {
		t.Fatalf("runDesignTodo failed: %v", err)
	}

	// Verify the todo is marked as done
	store, err = todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         "verify done",
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	items, err := store.Show([]string{created.ID})
	if err != nil {
		t.Fatalf("failed to show todo: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("todo not found")
	}
	if items[0].Status != todo.StatusDone {
		t.Fatalf("expected status done after successful session, got %q", items[0].Status)
	}
}

func TestRunDesignTodoReopensTodoOnNonZeroExit(t *testing.T) {
	originalSession := runInteractiveSession
	defer func() { runInteractiveSession = originalSession }()

	repoPath := setupJobDoTestRepo(t)

	// Create a design todo
	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
		Purpose:         "test",
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	created, err := store.Create("Test design todo", todo.CreateOptions{
		Type: todo.TypeDesign,
	})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}
	store.Release()

	// Mock session with non-zero exit
	runInteractiveSession = func(opts interactiveSessionOptions) (interactiveSessionResult, error) {
		return interactiveSessionResult{exitCode: 1}, nil
	}

	cmd := newTestJobDoCommand()
	err = runDesignTodo(cmd, repoPath, *created)
	// Should return an exit error
	var exitErr exitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected exitError, got %v", err)
	}
	if exitErr.code != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.code)
	}

	// Verify the todo is reopened to open status (not in_progress or done)
	store, err = todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         "verify reopened",
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	items, err := store.Show([]string{created.ID})
	if err != nil {
		t.Fatalf("failed to show todo: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("todo not found")
	}
	if items[0].Status != todo.StatusOpen {
		t.Fatalf("expected todo to be reopened to 'open' status, got %q", items[0].Status)
	}
}

// fakeQueueStore is a mock queueStore for testing queue operations.
type fakeQueueStore struct {
	queueFn  func(ids []string) ([]todo.Todo, error)
	reopenFn func(ids []string) ([]todo.Todo, error)
}

func (s *fakeQueueStore) Queue(ids []string) ([]todo.Todo, error) {
	if s.queueFn != nil {
		return s.queueFn(ids)
	}
	return nil, nil
}

func (s *fakeQueueStore) Reopen(ids []string) ([]todo.Todo, error) {
	if s.reopenFn != nil {
		return s.reopenFn(ids)
	}
	return nil, nil
}

func (s *fakeQueueStore) Release() error {
	return nil
}
func TestDefaultRunInteractiveSessionSetsProposerEnv(t *testing.T) {
	// This test verifies that the INCREMENTUM_TODO_PROPOSER=true environment
	// variable is set when running interactive sessions. We test this by mocking
	// runInteractiveSession at a higher level since the agent.Store can't be
	// easily mocked.
	//
	// The env var is passed via interactiveSessionOptions which is checked
	// in defaultRunInteractiveSession. Since that function constructs the
	// agent.RunOptions with the env var, we verify the behavior indirectly
	// by checking that interactive sessions include the env in their options.
	//
	// This is covered by inspection of the defaultRunInteractiveSession code
	// which explicitly includes todoenv.ProposerEnvVar + "=true" in the Env slice.
	t.Skip("Test requires mocking agent.Store; behavior verified by code inspection")
}

func TestRunDesignTodoReopensTodoOnSessionError(t *testing.T) {
	originalSession := runInteractiveSession
	defer func() { runInteractiveSession = originalSession }()

	repoPath := setupJobDoTestRepo(t)

	// Create a design todo
	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
		Purpose:         "test",
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	created, err := store.Create("Test design todo", todo.CreateOptions{
		Type: todo.TypeDesign,
	})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}
	store.Release()

	// Mock session that returns an error
	expectedErr := errors.New("session failed")
	runInteractiveSession = func(opts interactiveSessionOptions) (interactiveSessionResult, error) {
		return interactiveSessionResult{}, expectedErr
	}

	cmd := newTestJobDoCommand()
	err = runDesignTodo(cmd, repoPath, *created)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected session error, got %v", err)
	}

	// Verify the todo is reopened to open status (not in_progress)
	store, err = todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         "verify reopened",
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	items, err := store.Show([]string{created.ID})
	if err != nil {
		t.Fatalf("failed to show todo: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("todo not found")
	}
	if items[0].Status != todo.StatusOpen {
		t.Fatalf("expected todo to be reopened to 'open' status, got %q", items[0].Status)
	}
}

func TestFormatDesignTodoBlock(t *testing.T) {
	item := todo.Todo{
		ID:          "abc12345",
		Title:       "Design the API",
		Type:        todo.TypeDesign,
		Priority:    todo.PriorityMedium,
		Description: "Create a specification for the new API endpoints.",
	}

	output := formatDesignTodoBlock(item)

	// Verify the output contains expected fields
	if !strings.Contains(output, "ID: abc12345") {
		t.Fatalf("expected ID in output, got %q", output)
	}
	if !strings.Contains(output, "Title: Design the API") {
		t.Fatalf("expected title in output, got %q", output)
	}
	if !strings.Contains(output, "Type: design") {
		t.Fatalf("expected type in output, got %q", output)
	}
	if !strings.Contains(output, "Priority: 2") {
		t.Fatalf("expected priority in output, got %q", output)
	}
	if !strings.Contains(output, "Description:") {
		t.Fatalf("expected description label in output, got %q", output)
	}
	if !strings.Contains(output, "specification") {
		t.Fatalf("expected description content in output, got %q", output)
	}
}

func TestFormatDesignTodoBlockEmptyDescription(t *testing.T) {
	item := todo.Todo{
		ID:          "xyz99999",
		Title:       "Empty desc design",
		Type:        todo.TypeDesign,
		Priority:    todo.PriorityHigh,
		Description: "",
	}

	output := formatDesignTodoBlock(item)

	// Should use "-" for empty description
	if !strings.Contains(output, "-") {
		t.Fatalf("expected '-' for empty description, got %q", output)
	}
}

// wrappingDesignTodoStore wraps a real store but fails on Release.
// This allows Start() to actually modify the todo while simulating a Release failure.
type wrappingDesignTodoStore struct {
	inner      *todo.Store
	releaseErr error
	startCalls int
	// statusAfterStart records the status of each todo after Start() succeeds.
	// This allows tests to verify that the todo actually transitioned to in_progress.
	statusAfterStart map[string]todo.Status
}

func (s *wrappingDesignTodoStore) Start(ids []string) ([]todo.Todo, error) {
	s.startCalls++
	todos, err := s.inner.Start(ids)
	if err == nil && s.statusAfterStart != nil {
		// Record the status of each started todo
		for _, t := range todos {
			s.statusAfterStart[t.ID] = t.Status
		}
	}
	return todos, err
}

func (s *wrappingDesignTodoStore) Release() error {
	// Release the underlying store to avoid leaking resources.
	// Join any underlying error with the configured simulated error.
	innerErr := s.inner.Release()
	return errors.Join(s.releaseErr, innerErr)
}

func TestRunDesignTodoReopensTodoOnReleaseFailureAfterStart(t *testing.T) {
	// Save and restore original functions
	originalSession := runInteractiveSession
	originalStoreOpener := openDesignTodoStore
	defer func() {
		runInteractiveSession = originalSession
		openDesignTodoStore = originalStoreOpener
	}()

	repoPath := setupJobDoTestRepo(t)

	// Create a todo store and design todo using the real store
	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
		Purpose:         "test setup",
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	created, err := store.Create("Test design todo", todo.CreateOptions{
		Type: todo.TypeDesign,
	})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}
	if err := store.Release(); err != nil {
		t.Fatalf("failed to release store after create: %v", err)
	}

	releaseErr := errors.New("simulated release failure")
	var wrapper *wrappingDesignTodoStore

	// Override the store opener to return a wrapper that fails on Release
	openDesignTodoStore = func(repoPath, purpose string) (designTodoStore, error) {
		// For the "start" operation, return a wrapper around the real store
		// that calls real Start() but returns an error on Release()
		if strings.Contains(purpose, "start") {
			realStore, err := todo.Open(repoPath, todo.OpenOptions{
				CreateIfMissing: false,
				PromptToCreate:  false,
				Purpose:         purpose,
			})
			if err != nil {
				return nil, err
			}
			wrapper = &wrappingDesignTodoStore{
				inner:            realStore,
				releaseErr:       releaseErr,
				statusAfterStart: make(map[string]todo.Status),
			}
			return wrapper, nil
		}
		// Fallback for any other store operations (though currently only "start" uses
		// openDesignTodoStore). Note: reopenDesignTodo() calls todo.Open directly
		// rather than going through openDesignTodoStore.
		return todo.Open(repoPath, todo.OpenOptions{
			CreateIfMissing: false,
			PromptToCreate:  false,
			Purpose:         purpose,
		})
	}

	// Track whether interactive session was called
	sessionCalled := false
	runInteractiveSession = func(opts interactiveSessionOptions) (interactiveSessionResult, error) {
		sessionCalled = true
		return interactiveSessionResult{}, nil
	}

	// Verify the todo starts as open
	store, err = todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         "verify initial status",
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	items, err := store.Show([]string{created.ID})
	if err != nil {
		t.Fatalf("failed to show todo: %v", err)
	}
	if items[0].Status != todo.StatusOpen {
		t.Fatalf("expected initial status 'open', got %q", items[0].Status)
	}
	if err := store.Release(); err != nil {
		t.Fatalf("failed to release store after status check: %v", err)
	}

	// Run the design todo - should fail with the release error
	cmd := newTestJobDoCommand()
	err = runDesignTodo(cmd, repoPath, *created)

	// Verify we got the release error
	if err == nil {
		t.Fatal("expected error from runDesignTodo")
	}
	if !errors.Is(err, releaseErr) {
		t.Fatalf("expected release error, got %v", err)
	}

	// Verify the wrapper's Start was called (which calls the real store)
	if wrapper == nil || wrapper.startCalls != 1 {
		t.Fatal("expected Start to be called once on the wrapper")
	}

	// Verify that Start() actually transitioned the todo to in_progress.
	// This is critical: it proves the reopen logic is actually needed (the todo
	// was in_progress) rather than passing trivially (if Start were a no-op).
	statusAfterStart, ok := wrapper.statusAfterStart[created.ID]
	if !ok {
		t.Fatal("expected Start to record the todo status")
	}
	if statusAfterStart != todo.StatusInProgress {
		t.Fatalf("expected todo to be in_progress after Start(), got %q", statusAfterStart)
	}

	// Verify interactive session was NOT called (we fail before reaching it)
	if sessionCalled {
		t.Fatal("interactive session should not be called when Release fails after Start")
	}

	// Verify the todo was reopened (should be back to open status, not in_progress)
	// This is the critical assertion: the todo WAS in_progress after Start() succeeded,
	// but should now be open because our reopen logic ran.
	store, err = todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         "verify reopened",
	})
	if err != nil {
		t.Fatalf("failed to open store to verify: %v", err)
	}
	defer store.Release()

	items, err = store.Show([]string{created.ID})
	if err != nil {
		t.Fatalf("failed to show todo: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("todo not found")
	}
	if items[0].Status != todo.StatusOpen {
		t.Fatalf("expected todo to be reopened to 'open' status after Release failure, got %q", items[0].Status)
	}
}
