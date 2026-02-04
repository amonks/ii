package main

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/amonks/incrementum/internal/jj"
	jobpkg "github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
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

func TestFormatCommitMessagesOutputPreservesIndentation(t *testing.T) {
	entries := []jobpkg.CommitLogEntry{{
		ID:      "commit-123",
		Message: "Summary line\n\nHere is a generated commit message:\n\n    Body line\n\nThis commit is a step towards implementing this todo:\n\n    ID: todo-1",
	}}

	output := formatCommitMessagesOutput(entries)
	if !strings.Contains(output, "Commit messages:") {
		t.Fatalf("expected header, got %q", output)
	}
	if !strings.Contains(output, "    Commit commit-123:") {
		t.Fatalf("expected commit id label, got %q", output)
	}
	if !strings.Contains(output, "\n        Summary line") {
		t.Fatalf("expected summary line indentation, got %q", output)
	}
	if !strings.Contains(output, "\n            Body line") {
		t.Fatalf("expected body line indentation, got %q", output)
	}
	if !strings.Contains(output, "\n            ID: todo-1") {
		t.Fatalf("expected commit message indentation preserved, got %q", output)
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
	defer func() {
		jobDoTodo = originalJobDoTodo
	}()

	var got []string
	jobDoTodo = func(cmd *cobra.Command, todoID string) error {
		got = append(got, todoID)
		return nil
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
}

func TestJobDoAgentDefaultsToInternal(t *testing.T) {
	resetJobDoGlobals()
	cmd := newTestJobDoCommand()

	kind, err := parseJobDoAgentKind(cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if kind != jobAgentInternal {
		t.Fatalf("expected internal agent, got %q", kind)
	}
}

func TestJobDoAgentAllowsClaude(t *testing.T) {
	resetJobDoGlobals()
	cmd := newTestJobDoCommand()
	if err := cmd.Flags().Set("agent", "claude"); err != nil {
		t.Fatalf("set agent flag: %v", err)
	}

	kind, err := parseJobDoAgentKind(cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if kind != jobAgentClaude {
		t.Fatalf("expected claude agent, got %q", kind)
	}
}

func TestJobDoAgentRejectsUnknown(t *testing.T) {
	resetJobDoGlobals()
	cmd := newTestJobDoCommand()
	if err := cmd.Flags().Set("agent", "unknown"); err != nil {
		t.Fatalf("set agent flag: %v", err)
	}

	if _, err := parseJobDoAgentKind(cmd); err == nil {
		t.Fatal("expected error for unknown agent")
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
	jobDoAgent = ""
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
	cmd.Flags().StringVar(&jobDoAgent, "agent", "", "Agent backend (internal, claude, codex)")
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

func TestRunDesignTodoDoesNotMarkDoneOnNonZeroExit(t *testing.T) {
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

	// Verify the todo is NOT marked as done
	store, err = todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		Purpose:         "verify not done",
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
	if items[0].Status == todo.StatusDone {
		t.Fatal("expected todo NOT to be marked as done after failed session")
	}
}

func TestRunDesignTodoReturnsSessionError(t *testing.T) {
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
