package job

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalstrings "monks.co/incrementum/internal/strings"
	"monks.co/incrementum/todo"
)

func TestReadCommitMessageDeletesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commit-message")
	if err := os.WriteFile(path, []byte("feat: add widgets\n"), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	message, err := readCommitMessage(path)
	if err != nil {
		t.Fatalf("read commit message: %v", err)
	}
	if message != "feat: add widgets" {
		t.Fatalf("expected message %q, got %q", "feat: add widgets", message)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected commit message file to be deleted")
	}
}

func TestReadCommitMessageTrimsLeadingBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commit-message")
	contents := "\n\nfeat: add widgets    \n\nExplain the change.\t\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	message, err := readCommitMessage(path)
	if err != nil {
		t.Fatalf("read commit message: %v", err)
	}

	expected := "feat: add widgets\n\nExplain the change."
	if message != expected {
		t.Fatalf("expected message %q, got %q", expected, message)
	}
}

func TestReadCommitMessageDeletesEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commit-message")
	if err := os.WriteFile(path, []byte("\n"), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	if _, err := readCommitMessage(path); err == nil {
		t.Fatalf("expected error for empty commit message")
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected commit message file to be deleted")
	}
}

func TestReadCommitMessageMissingExplainsPath(t *testing.T) {
	workspaceDir := t.TempDir()
	primary := filepath.Join(workspaceDir, commitMessageFilename)

	_, err := readCommitMessage(primary)
	if err == nil {
		t.Fatal("expected missing commit message error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing file error, got %v", err)
	}
	if !strings.Contains(err.Error(), "commit message missing") {
		t.Fatalf("expected context in error, got %v", err)
	}
	if !strings.Contains(err.Error(), primary) {
		t.Fatalf("expected paths in error, got %v", err)
	}
}

func TestFormatCommitMessage(t *testing.T) {
	item := todo.Todo{
		ID:          "todo-555",
		Title:       "Shore up logs",
		Description: "Improve log formatting for the job runner.",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityLow,
	}
	message := "feat: reflow logs\n\nEnsure log output is wrapped and indented for clarity."

	formatted := formatCommitMessage(item, message, "")
	if !strings.HasPrefix(formatted, "feat: reflow logs") {
		t.Fatalf("expected commit summary at start, got %q", formatted)
	}

	checks := []string{
		"Here is a generated change description:",
		"    Ensure log output is wrapped and indented for clarity.",
		"This change is part of a series implementing this todo:",
		"    ID: todo-555",
		"    Title: Shore up logs",
		"    Type: task",
		"    Priority: 3 (low)",
		"    Description:",
		"        Improve log formatting for the job runner.",
	}
	for _, check := range checks {
		if !strings.Contains(formatted, check) {
			t.Fatalf("expected commit message to include %q, got %q", check, formatted)
		}
	}
}

func TestFormatCommitMessageNormalizesOutput(t *testing.T) {
	item := todo.Todo{
		ID:          "todo-909",
		Title:       "Normalize commit output",
		Description: "Ensure formatted commit output trims leading whitespace.",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityLow,
	}
	message := "\n\nfeat: add widgets    \n\nBody line\t\n"

	formatted := formatCommitMessage(item, message, "")
	lines := strings.Split(formatted, "\n")
	if lines[0] != "feat: add widgets" {
		t.Fatalf("expected normalized summary, got %q", lines[0])
	}
	if internalstrings.TrimTrailingWhitespace(lines[0]) != lines[0] {
		t.Fatalf("expected summary to have no trailing whitespace, got %q", lines[0])
	}
}

func TestFormatCommitMessageWithWidthRespectsLimit(t *testing.T) {
	item := todo.Todo{
		ID:          "todo-987",
		Title:       "Constrain log message widths",
		Description: "Ensure log formatting keeps nested sections within the requested width while preserving hierarchy.",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}
	message := "feat: narrow commit message output width for logs\n\nThis body text should wrap to fit within the chosen width, even when the summary and description are long."

	formatted := formatCommitMessageWithWidth(item, message, "", 60)
	if maxLineLength(formatted) > 60 {
		t.Fatalf("expected max line length <= 60, got %d", maxLineLength(formatted))
	}
}

func TestFormatCommitMessageRendersMarkdownBody(t *testing.T) {
	item := todo.Todo{
		ID:          "todo-456",
		Title:       "Render markdown",
		Description: "Ensure log formatting honors markdown sections.",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityLow,
	}
	message := "feat: render markdown\n\n- First item\n- Second item"

	formatted := formatCommitMessageWithWidth(item, message, "", 80)
	checks := []string{
		"    - First item",
		"    - Second item",
	}
	for _, check := range checks {
		if !strings.Contains(formatted, check) {
			t.Fatalf("expected formatted message to include %q, got %q", check, formatted)
		}
	}
}

func TestFormatCommitMessageWithReviewComments(t *testing.T) {
	item := todo.Todo{
		ID:          "todo-789",
		Title:       "Add review comments",
		Description: "Include reviewer comments in commit messages.",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}
	message := "feat: add review section\n\nAdded support for review comments."
	reviewComments := "Clean implementation, good test coverage."

	formatted := formatCommitMessage(item, message, reviewComments)
	if !strings.Contains(formatted, "Review comments:") {
		t.Fatalf("expected commit message to include review comments section, got %q", formatted)
	}
	if !strings.Contains(formatted, "    Clean implementation, good test coverage.") {
		t.Fatalf("expected commit message to include review text indented, got %q", formatted)
	}
}

func TestFormatCommitMessageEmptyReviewComments(t *testing.T) {
	item := todo.Todo{
		ID:          "todo-321",
		Title:       "No review comments",
		Description: "Should not include empty review section.",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityLow,
	}
	message := "feat: no review\n\nBasic change."

	formatted := formatCommitMessage(item, message, "")
	if strings.Contains(formatted, "Review comments:") {
		t.Fatalf("expected commit message to not include review comments section when empty, got %q", formatted)
	}
}

func maxLineLength(value string) int {
	max := 0
	for line := range strings.SplitSeq(value, "\n") {
		length := len(line)
		if length > max {
			max = length
		}
	}
	return max
}
