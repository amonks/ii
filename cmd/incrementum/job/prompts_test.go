package job

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalstrings "monks.co/incrementum/internal/strings"
	"monks.co/incrementum/todo"
)

func TestLoadPrompt_UsesOverride(t *testing.T) {
	repoPath := t.TempDir()
	promptDir := filepath.Join(repoPath, ".incrementum", "templates")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}

	override := "override content"
	overridePath := filepath.Join(promptDir, "prompt-implementation.tmpl")
	if err := os.WriteFile(overridePath, []byte(override), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	loaded, err := LoadPrompt(repoPath, "prompt-implementation.tmpl")
	if err != nil {
		t.Fatalf("load prompt: %v", err)
	}

	if trimmedPromptOutputNoTrailingSpaces(loaded) != override {
		t.Fatalf("expected override content, got %q", loaded)
	}
}

func TestLoadPrompt_UsesEmbeddedDefault(t *testing.T) {
	repoPath := t.TempDir()

	loaded, err := LoadPrompt(repoPath, "prompt-commit-review.tmpl")
	if err != nil {
		t.Fatalf("load prompt: %v", err)
	}

	if !strings.Contains(loaded, "Review the change") {
		t.Fatalf("expected embedded prompt, got %q", loaded)
	}
}

func TestRenderPrompt_InterpolatesFields(t *testing.T) {
	data := PromptData{
		Todo: todo.Todo{
			ID:          "todo-123",
			Title:       "Ship it",
			Description: "Do the thing",
			Type:        todo.TypeTask,
			Priority:    todo.PriorityHigh,
		},
		Feedback:        "Needs more tests",
		Message:         "Add coverage",
		WorkflowContext: "Workflow context",
		ReviewQuestions: "Review questions",
	}

	rendered, err := RenderPrompt("", "{{.Todo.ID}} {{.Todo.Title}} {{.Feedback}} {{.Message}}", data)
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	expected := "todo-123 Ship it Needs more tests Add coverage"
	if trimmedPromptOutputNoTrailingSpaces(rendered) != expected {
		t.Fatalf("expected %q, got %q", expected, rendered)
	}
}

func TestRenderPrompt_InterpolatesWorkspacePath(t *testing.T) {
	data := PromptData{WorkspacePath: "/tmp/ws-123", WorkflowContext: "Workflow context", ReviewQuestions: "Review questions"}

	rendered, err := RenderPrompt("", "{{.WorkspacePath}}", data)
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	if trimmedPromptOutputNoTrailingSpaces(rendered) != "/tmp/ws-123" {
		t.Fatalf("expected workspace path to render, got %q", rendered)
	}
}

func TestRenderPrompt_InterpolatesReviewInstructions(t *testing.T) {
	data := PromptData{ReviewInstructions: "Follow the steps.", WorkflowContext: "Workflow context", ReviewQuestions: "Review questions"}

	rendered, err := RenderPrompt("", "{{.ReviewInstructions}}", data)
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	if trimmedPromptOutputNoTrailingSpaces(rendered) != "Follow the steps." {
		t.Fatalf("expected review instructions to render, got %q", rendered)
	}
}

func TestRenderPrompt_InterpolatesTodoBlock(t *testing.T) {
	data := PromptData{TodoBlock: "Todo\n\n    id", WorkflowContext: "Workflow context", ReviewQuestions: "Review questions"}

	rendered, err := RenderPrompt("", "{{.TodoBlock}}", data)
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	if trimmedPromptOutputNoTrailingSpaces(rendered) != "Todo\n\n    id" {
		t.Fatalf("expected todo block to render, got %q", rendered)
	}
}

func TestFormatTodoBlock_PreservesFieldLines(t *testing.T) {
	item := todo.Todo{
		ID:          "todo-123",
		Title:       "Ship it",
		Description: "Do the thing",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	formatted := formatTodoBlock(item)

	expected := strings.Join([]string{
		"Todo",
		"",
		"    ID: todo-123",
		"    Title: Ship it",
		"    Type: task",
		"    Priority: 2",
		"    Description:",
		"        Do the thing",
	}, "\n")

	if internalstrings.TrimTrailingNewlines(formatted) != expected {
		t.Fatalf("expected todo block fields to stay on separate lines, got %q", formatted)
	}
}

func TestFormatFeedbackBlock_PreservesListItems(t *testing.T) {
	feedback := strings.Join([]string{
		"- npm run lint is passing",
		"- npm run test is failing",
	}, "\n")

	formatted := formatFeedbackBlock(feedback)

	expected := strings.Join([]string{
		"Previous feedback",
		"",
		"    - npm run lint is passing",
		"    - npm run test is failing",
	}, "\n")

	if internalstrings.TrimTrailingNewlines(formatted) != expected {
		t.Fatalf("expected feedback list to stay on separate lines, got %q", formatted)
	}
}

func TestRenderPrompt_RendersReviewQuestionsTemplate(t *testing.T) {
	workflowTemplate, err := LoadPrompt("", reviewQuestionsTemplateName)
	if err != nil {
		t.Fatalf("load review questions: %v", err)
	}
	reviewQuestions, err := renderContextTemplate("review_questions", workflowTemplate)
	if err != nil {
		t.Fatalf("render review questions: %v", err)
	}

	rendered, err := RenderPrompt("", "{{template \"review_questions\"}}", PromptData{ReviewQuestions: reviewQuestions})
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	if !strings.Contains(rendered, "Does it do what the change description says?") {
		t.Fatalf("expected review questions to render, got %q", rendered)
	}
}

func TestRenderPrompt_RendersWorkflowContextTemplate(t *testing.T) {
	workflowTemplate, err := LoadPrompt("", workflowContextTemplateName)
	if err != nil {
		t.Fatalf("load workflow context: %v", err)
	}
	workflowContext, err := renderContextTemplate("workflow_context", workflowTemplate)
	if err != nil {
		t.Fatalf("render workflow context: %v", err)
	}

	rendered, err := RenderPrompt("", "{{template \"workflow_context\"}}", PromptData{WorkflowContext: workflowContext})
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	if !strings.Contains(rendered, "## Workflow Context") {
		t.Fatalf("expected workflow context to render, got %q", rendered)
	}
}

func TestBuildPromptParts_SplitsPhaseAndUserContent(t *testing.T) {
	context := PromptContext{WorkflowContext: "Workflow", ReviewQuestions: "Questions"}
	parts, err := buildPromptParts(
		todo.Todo{ID: "todo-1", Title: "Title", Type: todo.TypeTask, Priority: todo.PriorityHigh},
		"",
		"message",
		"series",
		nil,
		"/tmp",
		[]string{"go test ./..."},
		context,
		"{{.WorkflowContext}}\nPhase instructions\n{{.ReviewInstructions}}\n{{.ReviewQuestions}}",
	)
	if err != nil {
		t.Fatalf("build prompt parts: %v", err)
	}
	if parts.PhaseContent == "" || parts.UserContent == "" {
		t.Fatalf("expected phase and user content, got %#v", parts)
	}
	if !strings.Contains(parts.PhaseContent, "Phase instructions") {
		t.Fatalf("expected phase content to include instructions")
	}
	if !strings.Contains(parts.PhaseContent, "Workflow") {
		t.Fatalf("expected workflow context in phase content")
	}
	if !strings.Contains(parts.PhaseContent, "Questions") {
		t.Fatalf("expected review questions in phase content")
	}
	if len(parts.ProjectContext) == 0 || !strings.Contains(parts.ProjectContext[len(parts.ProjectContext)-1], "Publish your review") {
		t.Fatalf("expected review instructions in project context")
	}
}

func TestRenderPrompt_UsesReviewQuestionsOverride(t *testing.T) {
	repoPath := t.TempDir()
	promptDir := filepath.Join(repoPath, ".incrementum", "templates")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}

	override := "{{define \"review_questions\"}}- override{{end}}"
	overridePath := filepath.Join(promptDir, "review-questions.tmpl")
	if err := os.WriteFile(overridePath, []byte(override), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	reviewQuestions, err := renderContextTemplate("review_questions", override)
	if err != nil {
		t.Fatalf("render review questions: %v", err)
	}

	rendered, err := RenderPrompt(repoPath, "{{template \"review_questions\"}}", PromptData{ReviewQuestions: reviewQuestions})
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	if trimmedPromptOutputNoTrailingSpaces(rendered) != "- override" {
		t.Fatalf("expected override content, got %q", rendered)
	}
}
