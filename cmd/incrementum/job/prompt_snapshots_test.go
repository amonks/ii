package job

import (
	"path/filepath"
	"testing"

	"monks.co/incrementum/todo"
)

func TestPromptSnapshots(t *testing.T) {
	data, context, testCommands, seriesLog := promptSnapshotData(t)
	promptFiles := []string{
		"prompt-implementation.tmpl",
		"prompt-feedback.tmpl",
		"prompt-commit-review.tmpl",
		"prompt-project-review.tmpl",
	}

	for _, name := range promptFiles {
		t.Run(name, func(t *testing.T) {
			contents, err := LoadPrompt("", name)
			if err != nil {
				t.Fatalf("load prompt: %v", err)
			}
			parts, err := buildPromptParts(data.Todo, data.Feedback, data.Message, seriesLog, data.AgentTranscripts, data.WorkspacePath, testCommands, context, contents)
			if err != nil {
				t.Fatalf("build prompt parts: %v", err)
			}
			promptContent := promptContentFromParts(parts)
			if len(testCommands) > 0 {
				promptContent.TestCommands = testCommands
			}
			prompt := renderPromptLog(promptContent)
			snapshotName := name + ".txt"
			requireSnapshot(t, snapshotName, prompt)
		})
	}
}

func promptSnapshotData(t *testing.T) (PromptData, PromptContext, []string, string) {
	t.Helper()
	item := todo.Todo{
		ID:       "todo-57uzut5r",
		Title:    "Snapshot-test text formatting",
		Type:     todo.TypeTask,
		Priority: todo.PriorityHigh,
		Description: "Build snapshot tests for long-form output so regressions are obvious. " +
			"Cover prompt rendering, commit message formatting, and log snapshots. " +
			"Make sure wrapping handles long lines, bullets, and mixed indentation.\n\n" +
			"- First bullet item has a long line that should wrap within the todo description block and keep indentation consistent.\n" +
			"- Second bullet is shorter but still wraps when it needs to.\n\n" +
			"    Indented block line one should wrap and stay indented even when the line is long enough to exceed the width.\n" +
			"\n" +
			"    Indented block line two continues with more words to force another wrap and confirm spacing.",
	}

	feedback := "Reviewer notes:\n" +
		"- Verify wrapping in long paragraphs and list items.\n" +
		"- Ensure blank lines remain where expected.\n\n" +
		"Please double-check that empty lines are preserved between sections."

	message := "feat: snapshot text formatting\n\n" +
		"Add snapshot tests for prompts and commit messages, ensuring wrapping for long lines and bulleted lists stays consistent."

	seriesLog := "abc123 john@example.com 5 minutes ago main\n" +
		"Add snapshot tests for text formatting"

	workflowTemplate, err := LoadPrompt("", workflowContextTemplateName)
	if err != nil {
		t.Fatalf("load workflow context: %v", err)
	}
	workflowContext, err := renderContextTemplate("workflow_context", workflowTemplate)
	if err != nil {
		t.Fatalf("render workflow context: %v", err)
	}
	reviewTemplate, err := LoadPrompt("", reviewQuestionsTemplateName)
	if err != nil {
		t.Fatalf("load review questions: %v", err)
	}
	reviewQuestions, err := renderContextTemplate("review_questions", reviewTemplate)
	if err != nil {
		t.Fatalf("render review questions: %v", err)
	}
	context := PromptContext{WorkflowContext: "\n" + workflowContext + "\n", ReviewQuestions: reviewQuestions + "\n", ContextFiles: []string{"Project context."}}
	testCommands := []string{"go test ./...", "go vet ./..."}
	data := newPromptData(item, feedback, message, seriesLog, nil, filepath.Join("/tmp", "workspaces", "snapshot-test"), context)
	return data, context, testCommands, seriesLog
}
