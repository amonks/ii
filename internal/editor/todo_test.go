package editor

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	internalstrings "monks.co/ii/internal/strings"
	"monks.co/ii/todo"
)

func TestRenderTodoTOML_Create(t *testing.T) {
	data := DefaultCreateData()
	content, err := RenderTodoTOML(data)
	if err != nil {
		t.Fatalf("RenderTodoTOML failed: %v", err)
	}
	assertUnindentedFrontmatter(t, content)

	// Check required elements are present
	if !strings.Contains(content, `title = ""`) {
		t.Error("expected empty title")
	}
	if !strings.Contains(content, `type = "task"`) {
		t.Error("expected default type 'task'")
	}
	if !strings.Contains(content, "priority = 2") {
		t.Error("expected default priority 2")
	}
	if !strings.Contains(content, `status = "open"`) {
		t.Error("expected default status open")
	}
	if !strings.Contains(content, `implementation-model = ""`) {
		t.Error("expected default implementation-model empty")
	}
	if !strings.Contains(content, `code-review-model = ""`) {
		t.Error("expected default code-review-model empty")
	}
	if !strings.Contains(content, `project-review-model = ""`) {
		t.Error("expected default project-review-model empty")
	}
	if strings.Contains(content, "description =") {
		t.Error("expected description to be in body")
	}
	if !strings.Contains(content, "---") {
		t.Error("expected frontmatter separator")
	}

	if !strings.Contains(content, "proposed") {
		t.Error("expected status comment to mention proposed")
	}
}

func TestRenderTodoTOML_Update(t *testing.T) {
	existing := &todo.Todo{
		ID:                  "abc12345",
		Title:               "Test Todo",
		Type:                todo.TypeFeature,
		Priority:            todo.PriorityHigh,
		Status:              todo.StatusInProgress,
		Description:         "A test description",
		ImplementationModel: "impl-model",
		CodeReviewModel:     "review-model",
		ProjectReviewModel:  "project-model",
	}

	data := DataFromTodo(existing)
	content, err := RenderTodoTOML(data)
	if err != nil {
		t.Fatalf("RenderTodoTOML failed: %v", err)
	}
	assertUnindentedFrontmatter(t, content)

	// Check fields are present with values
	if !strings.Contains(content, `title = "Test Todo"`) {
		t.Error("expected title to be set")
	}
	if !strings.Contains(content, `type = "feature"`) {
		t.Error("expected type to be feature")
	}
	if !strings.Contains(content, "priority = 1") {
		t.Error("expected priority to be 1 (high)")
	}
	if !strings.Contains(content, `status = "in_progress"`) {
		t.Error("expected status to be in_progress")
	}
	if !strings.Contains(content, `implementation-model = "impl-model"`) {
		t.Error("expected implementation model to be set")
	}
	if !strings.Contains(content, `code-review-model = "review-model"`) {
		t.Error("expected code review model to be set")
	}
	if !strings.Contains(content, `project-review-model = "project-model"`) {
		t.Error("expected project review model to be set")
	}
	if !strings.Contains(content, "proposed") {
		t.Error("expected status comment to mention proposed")
	}
	if strings.Contains(content, "description =") {
		t.Error("expected description to be in body")
	}
	if !strings.Contains(content, "A test description") {
		t.Error("expected description content")
	}
}

func TestParseTodoTOML(t *testing.T) {
	content := `
 title = "My Todo"
 type = "bug"
 priority = 0
 status = "done"
 implementation-model = "impl"
 code-review-model = "review"
 project-review-model = "project"
 ---
 This is a description
 with multiple lines
 `

	parsed, err := ParseTodoTOML(content)
	if err != nil {
		t.Fatalf("ParseTodoTOML failed: %v", err)
	}

	if parsed.Title != "My Todo" {
		t.Errorf("expected title 'My Todo', got %q", parsed.Title)
	}
	if parsed.Type != "bug" {
		t.Errorf("expected type 'bug', got %q", parsed.Type)
	}
	if parsed.Priority != 0 {
		t.Errorf("expected priority 0, got %d", parsed.Priority)
	}
	if parsed.Status == nil || *parsed.Status != "done" {
		t.Errorf("expected status 'done', got %v", parsed.Status)
	}
	if parsed.ImplementationModel != "impl" {
		t.Errorf("expected implementation model 'impl', got %q", parsed.ImplementationModel)
	}
	if parsed.CodeReviewModel != "review" {
		t.Errorf("expected code review model 'review', got %q", parsed.CodeReviewModel)
	}
	if parsed.ProjectReviewModel != "project" {
		t.Errorf("expected project review model 'project', got %q", parsed.ProjectReviewModel)
	}
	if strings.Contains(parsed.Description, "description =") {
		t.Errorf("expected description body without key, got %q", parsed.Description)
	}
	if !strings.Contains(parsed.Description, "multiple lines") {
		t.Errorf("expected description with multiple lines, got %q", parsed.Description)
	}
}

func TestParseTodoTOML_NormalizesCase(t *testing.T) {
	content := `title = "My Todo"
type = "BUG"
priority = 1
status = "DONE"`

	parsed, err := ParseTodoTOML(content)
	if err != nil {
		t.Fatalf("ParseTodoTOML failed: %v", err)
	}

	if parsed.Type != "bug" {
		t.Errorf("expected type 'bug', got %q", parsed.Type)
	}
	if parsed.Status == nil || *parsed.Status != "done" {
		t.Errorf("expected status 'done', got %v", parsed.Status)
	}
}

func TestParseTodoTOML_Validation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "missing title",
			content: `type = "task"`,
			wantErr: "title cannot be empty",
		},
		{
			name:    "invalid type",
			content: `title = "test"` + "\n" + `type = "invalid"`,
			wantErr: "invalid type",
		},
		{
			name: "invalid priority",
			content: `title = "test"
type = "task"
priority = 10`,
			wantErr: "priority",
		},
		{
			name: "invalid status",
			content: `title = "test"
type = "task"
priority = 2
status = "bad"`,
			wantErr: "invalid status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTodoTOML(tt.content)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestParseTodoTOML_InvalidStatusMentionsTombstone(t *testing.T) {
	content := `title = "test"
type = "task"
priority = 2
status = "bad"`

	_, err := ParseTodoTOML(content)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "proposed") {
		t.Errorf("expected error to mention proposed, got %q", err.Error())
	}
}

func TestToCreateOptions(t *testing.T) {
	status := "proposed"
	parsed := &ParsedTodo{
		Title:               "Test",
		Type:                "feature",
		Priority:            1,
		Status:              &status,
		Description:         "description",
		ImplementationModel: "impl",
		CodeReviewModel:     "review",
		ProjectReviewModel:  "project",
	}

	opts := parsed.ToCreateOptions()

	if opts.Type != todo.TypeFeature {
		t.Errorf("expected type feature, got %v", opts.Type)
	}
	if opts.Priority == nil || *opts.Priority != 1 {
		t.Errorf("expected priority 1, got %v", opts.Priority)
	}
	if opts.Description != "description" {
		t.Errorf("expected description 'description', got %q", opts.Description)
	}
	if opts.ImplementationModel != "impl" {
		t.Errorf("expected implementation model 'impl', got %q", opts.ImplementationModel)
	}
	if opts.CodeReviewModel != "review" {
		t.Errorf("expected code review model 'review', got %q", opts.CodeReviewModel)
	}
	if opts.ProjectReviewModel != "project" {
		t.Errorf("expected project review model 'project', got %q", opts.ProjectReviewModel)
	}
	if opts.Status != todo.StatusProposed {
		t.Errorf("expected status proposed, got %v", opts.Status)
	}
}

func TestToUpdateOptions(t *testing.T) {
	status := "in_progress"
	parsed := &ParsedTodo{
		Title:               "Test",
		Type:                "bug",
		Priority:            2,
		Status:              &status,
		Description:         "description",
		ImplementationModel: "impl",
		CodeReviewModel:     "review",
		ProjectReviewModel:  "project",
	}

	opts := parsed.ToUpdateOptions()

	if opts.Title == nil || *opts.Title != "Test" {
		t.Errorf("expected title 'Test', got %v", opts.Title)
	}
	if opts.Type == nil || *opts.Type != todo.TypeBug {
		t.Errorf("expected type bug, got %v", opts.Type)
	}
	if opts.Priority == nil || *opts.Priority != 2 {
		t.Errorf("expected priority 2, got %v", opts.Priority)
	}
	if opts.ImplementationModel == nil || *opts.ImplementationModel != "impl" {
		t.Errorf("expected implementation model 'impl', got %v", opts.ImplementationModel)
	}
	if opts.CodeReviewModel == nil || *opts.CodeReviewModel != "review" {
		t.Errorf("expected code review model 'review', got %v", opts.CodeReviewModel)
	}
	if opts.ProjectReviewModel == nil || *opts.ProjectReviewModel != "project" {
		t.Errorf("expected project review model 'project', got %v", opts.ProjectReviewModel)
	}
	if opts.Status == nil || *opts.Status != todo.StatusInProgress {
		t.Errorf("expected status in_progress, got %v", opts.Status)
	}
}

func TestCreateTodoTempFileExtension(t *testing.T) {
	file, err := createTodoTempFile()
	if err != nil {
		t.Fatalf("createTodoTempFile failed: %v", err)
	}
	t.Cleanup(func() {
		os.Remove(file.Name())
	})

	if !strings.HasSuffix(file.Name(), ".md") {
		t.Errorf("expected temp file to end with .md, got %q", file.Name())
	}
}

func assertUnindentedFrontmatter(t *testing.T, content string) {
	t.Helper()
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		if internalstrings.IsBlank(line) {
			continue
		}
		if isFrontmatterSeparator(line) {
			break
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			t.Fatalf("expected frontmatter line to be unindented, got %q", line)
		}
	}
}

// mockPrompter implements todo.Prompter for testing.
type mockPrompter struct {
	confirmResult bool
	confirmErr    error
	confirmCalled bool
	callCount     int
}

func (m *mockPrompter) Confirm(message string) (bool, error) {
	m.confirmCalled = true
	m.callCount++
	return m.confirmResult, m.confirmErr
}

// multiResponsePrompter returns different responses on successive calls.
type multiResponsePrompter struct {
	responses []bool
	errors    []error
	callCount int
}

func (m *multiResponsePrompter) Confirm(message string) (bool, error) {
	idx := m.callCount
	m.callCount++
	if idx < len(m.responses) {
		var err error
		if idx < len(m.errors) {
			err = m.errors[idx]
		}
		return m.responses[idx], err
	}
	return false, nil
}

// TestEditTodoWithDataRetry_NoPrompter_DeletesTempFile verifies that when prompter is nil
// (non-interactive use), the temp file is deleted even on parse errors.
// This maintains backward compatibility with EditTodoWithData.
//
// Note: This test mutates EDITOR and os.Stderr globals, so it is not parallel-safe.
func TestEditTodoWithDataRetry_NoPrompter_DeletesTempFile(t *testing.T) {
	// Create a file to record the temp file path passed to the editor
	pathRecordFile, err := os.CreateTemp("", "editor-path-record-*.txt")
	if err != nil {
		t.Fatalf("create path record file: %v", err)
	}
	pathRecordFile.Close()
	defer os.Remove(pathRecordFile.Name())

	// Create a script that writes invalid TOML to the file and records the path
	scriptContent := fmt.Sprintf(`#!/bin/sh
echo "$1" > "%s"
echo "invalid toml {{{{" > "$1"
`, pathRecordFile.Name())
	scriptFile, err := os.CreateTemp("", "test-editor-*.sh")
	if err != nil {
		t.Fatalf("create script file: %v", err)
	}
	defer os.Remove(scriptFile.Name())
	if _, err := scriptFile.WriteString(scriptContent); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := scriptFile.Chmod(0755); err != nil {
		t.Fatalf("chmod script: %v", err)
	}
	scriptFile.Close()

	// Set EDITOR to our script
	oldEditor := os.Getenv("EDITOR")
	os.Setenv("EDITOR", scriptFile.Name())
	defer os.Setenv("EDITOR", oldEditor)

	// Capture stderr to verify NO "saved to" message is printed
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	data := DefaultCreateData()
	_, err = EditTodoWithDataRetry(data, nil)

	w.Close()
	os.Stderr = oldStderr
	var stderrBuf strings.Builder
	io.Copy(&stderrBuf, r)
	stderrOutput := stderrBuf.String()

	if err == nil {
		t.Fatal("expected error when prompter is nil and parsing fails")
	}
	// The error should be a parse error (TOML syntax error)
	if !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "TOML") {
		t.Errorf("expected parse/TOML error, got: %v", err)
	}

	// When prompter is nil (non-interactive), we should NOT print a "saved to" message
	// and the temp file should be deleted (matching prior EditTodoWithData behavior)
	if strings.Contains(stderrOutput, "Your work has been saved to:") {
		t.Errorf("should NOT print 'saved to' message when prompter is nil, got: %q", stderrOutput)
	}

	// Verify the temp file was actually deleted
	recordedPath, readErr := os.ReadFile(pathRecordFile.Name())
	if readErr != nil {
		t.Fatalf("read path record file: %v", readErr)
	}
	tempPath := strings.TrimSpace(string(recordedPath))
	if tempPath == "" {
		t.Fatal("editor script did not record the temp file path")
	}
	if _, statErr := os.Stat(tempPath); !os.IsNotExist(statErr) {
		t.Errorf("temp file should have been deleted but still exists: %s", tempPath)
		os.Remove(tempPath) // Clean up if test fails
	}
}

// TestEditTodoWithDataRetry_PrompterReturnsFalse_DeletesTempFile verifies that when the user
// explicitly declines to retry, the temp file is deleted (respecting their decision to abandon).
//
// Note: This test mutates EDITOR and os.Stderr globals, so it is not parallel-safe.
func TestEditTodoWithDataRetry_PrompterReturnsFalse_DeletesTempFile(t *testing.T) {
	// Create a file to record the temp file path passed to the editor
	pathRecordFile, err := os.CreateTemp("", "editor-path-record-*.txt")
	if err != nil {
		t.Fatalf("create path record file: %v", err)
	}
	pathRecordFile.Close()
	defer os.Remove(pathRecordFile.Name())

	// Create a script that writes invalid TOML to the file and records the path
	scriptContent := fmt.Sprintf(`#!/bin/sh
echo "$1" > "%s"
echo "title = \"\"" > "$1"
`, pathRecordFile.Name())
	scriptFile, err := os.CreateTemp("", "test-editor-*.sh")
	if err != nil {
		t.Fatalf("create script file: %v", err)
	}
	defer os.Remove(scriptFile.Name())
	if _, err := scriptFile.WriteString(scriptContent); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := scriptFile.Chmod(0755); err != nil {
		t.Fatalf("chmod script: %v", err)
	}
	scriptFile.Close()

	// Set EDITOR to our script
	oldEditor := os.Getenv("EDITOR")
	os.Setenv("EDITOR", scriptFile.Name())
	defer os.Setenv("EDITOR", oldEditor)

	// Capture stderr to verify NO "saved to" message is printed
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	prompter := &mockPrompter{
		confirmResult: false,
		confirmErr:    nil,
	}

	data := DefaultCreateData()
	_, err = EditTodoWithDataRetry(data, prompter)

	w.Close()
	os.Stderr = oldStderr
	var stderrBuf strings.Builder
	io.Copy(&stderrBuf, r)
	stderrOutput := stderrBuf.String()

	if err == nil {
		t.Fatal("expected error when prompter returns false")
	}
	if !prompter.confirmCalled {
		t.Error("expected prompter.Confirm to be called")
	}
	// The error should be a validation error (empty title)
	if !strings.Contains(err.Error(), "title") {
		t.Errorf("expected title validation error, got: %v", err)
	}

	// When user declines retry, the "saved to" message should NOT be printed
	// because we respect their decision to abandon the edit
	if strings.Contains(stderrOutput, "Your work has been saved to:") {
		t.Errorf("should NOT print 'saved to' message when user declines retry, got: %q", stderrOutput)
	}

	// Verify the temp file was actually deleted
	recordedPath, readErr := os.ReadFile(pathRecordFile.Name())
	if readErr != nil {
		t.Fatalf("read path record file: %v", readErr)
	}
	tempPath := strings.TrimSpace(string(recordedPath))
	if tempPath == "" {
		t.Fatal("editor script did not record the temp file path")
	}
	if _, statErr := os.Stat(tempPath); !os.IsNotExist(statErr) {
		t.Errorf("temp file should have been deleted but still exists: %s", tempPath)
		os.Remove(tempPath) // Clean up if test fails
	}
}

// TestEditTodoWithDataRetry_PrompterReturnsError_PreservesTempFile verifies that when the prompter
// returns an error (e.g., io.EOF from stdin), the temp file is preserved so the user can recover.
//
// Note: This test mutates EDITOR and os.Stderr globals, so it is not parallel-safe.
func TestEditTodoWithDataRetry_PrompterReturnsError_PreservesTempFile(t *testing.T) {
	// Create a script that writes invalid TOML to the file
	scriptContent := `#!/bin/sh
echo "title = \"\"" > "$1"
`
	scriptFile, err := os.CreateTemp("", "test-editor-*.sh")
	if err != nil {
		t.Fatalf("create script file: %v", err)
	}
	defer os.Remove(scriptFile.Name())
	if _, err := scriptFile.WriteString(scriptContent); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := scriptFile.Chmod(0755); err != nil {
		t.Fatalf("chmod script: %v", err)
	}
	scriptFile.Close()

	// Set EDITOR to our script
	oldEditor := os.Getenv("EDITOR")
	os.Setenv("EDITOR", scriptFile.Name())
	defer os.Setenv("EDITOR", oldEditor)

	// Capture stderr to check for the "saved to" message
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Simulate EOF from stdin when reading the confirmation prompt
	prompter := &mockPrompter{
		confirmResult: false,
		confirmErr:    io.EOF,
	}

	data := DefaultCreateData()
	_, err = EditTodoWithDataRetry(data, prompter)

	w.Close()
	os.Stderr = oldStderr
	var stderrBuf strings.Builder
	io.Copy(&stderrBuf, r)
	stderrOutput := stderrBuf.String()

	if err == nil {
		t.Fatal("expected error when prompter returns error")
	}
	if !prompter.confirmCalled {
		t.Error("expected prompter.Confirm to be called")
	}
	// The error should wrap the prompt error (EOF), not the parse error.
	// This makes it clear that we aborted because the recovery prompt failed.
	if !strings.Contains(err.Error(), "prompt") {
		t.Errorf("expected error to contain 'prompt', got: %v", err)
	}
	if !strings.Contains(err.Error(), "EOF") {
		t.Errorf("expected error to contain 'EOF', got: %v", err)
	}

	// When prompter returns an error (e.g., EOF), we preserve the file
	// and print the path so the user can recover their work
	if !strings.Contains(stderrOutput, "Your work has been saved to:") {
		t.Errorf("expected 'saved to' message in stderr, got: %q", stderrOutput)
	}

	// Extract the temp file path from the message and verify it exists
	prefix := "Your work has been saved to: "
	idx := strings.Index(stderrOutput, prefix)
	if idx >= 0 {
		pathStart := idx + len(prefix)
		pathEnd := strings.Index(stderrOutput[pathStart:], "\n")
		if pathEnd == -1 {
			pathEnd = len(stderrOutput) - pathStart
		}
		tempPath := stderrOutput[pathStart : pathStart+pathEnd]
		if _, statErr := os.Stat(tempPath); os.IsNotExist(statErr) {
			t.Errorf("temp file should have been preserved but does not exist: %s", tempPath)
		} else {
			// Clean up the preserved temp file
			os.Remove(tempPath)
		}
	}
}

// TestEditTodoWithDataRetry_RetrySuccess verifies that when the user accepts the retry prompt,
// the editor is re-opened and parsing succeeds on the corrected content.
//
// Note: This test mutates EDITOR and os.Stderr globals, so it is not parallel-safe.
func TestEditTodoWithDataRetry_RetrySuccess(t *testing.T) {
	// Create a state file that tracks invocation count
	stateFile, err := os.CreateTemp("", "editor-state-*.txt")
	if err != nil {
		t.Fatalf("create state file: %v", err)
	}
	stateFile.WriteString("0")
	stateFile.Close()
	defer os.Remove(stateFile.Name())

	// Create a script that:
	// - On first invocation: writes invalid content (empty title)
	// - On second invocation: writes valid content
	scriptContent := fmt.Sprintf(`#!/bin/sh
COUNT=$(cat "%s")
if [ "$COUNT" = "0" ]; then
    echo "1" > "%s"
    # Write invalid content (empty title)
    cat > "$1" << 'EOF'
title = ""
type = "task"
status = "open"
priority = 2
---
Invalid on first try
EOF
else
    # Write valid content
    cat > "$1" << 'EOF'
title = "Valid Todo Title"
type = "task"
status = "open"
priority = 2
---
This is a valid todo description.
EOF
fi
`, stateFile.Name(), stateFile.Name())

	scriptFile, err := os.CreateTemp("", "test-editor-*.sh")
	if err != nil {
		t.Fatalf("create script file: %v", err)
	}
	defer os.Remove(scriptFile.Name())
	if _, err := scriptFile.WriteString(scriptContent); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := scriptFile.Chmod(0755); err != nil {
		t.Fatalf("chmod script: %v", err)
	}
	scriptFile.Close()

	// Set EDITOR to our script
	oldEditor := os.Getenv("EDITOR")
	os.Setenv("EDITOR", scriptFile.Name())
	defer os.Setenv("EDITOR", oldEditor)

	// Prompter returns true (retry) on first call
	prompter := &multiResponsePrompter{
		responses: []bool{true},
	}

	data := DefaultCreateData()
	parsed, err := EditTodoWithDataRetry(data, prompter)

	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if prompter.callCount != 1 {
		t.Errorf("expected prompter to be called once, got %d calls", prompter.callCount)
	}
	if parsed.Title != "Valid Todo Title" {
		t.Errorf("expected title 'Valid Todo Title', got %q", parsed.Title)
	}
	if parsed.Description != "This is a valid todo description.\n" {
		t.Errorf("expected description with trailing newline, got %q", parsed.Description)
	}
}
