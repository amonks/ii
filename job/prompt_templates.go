package job

import (
	"fmt"
	"path/filepath"
)

// PromptTemplateVariable documents a template variable and its Go type.
type PromptTemplateVariable struct {
	Name string
	Type string
}

// PromptTemplateInfo documents a default prompt template.
type PromptTemplateInfo struct {
	Name      string
	Contents  string
	Variables []PromptTemplateVariable
}

// PromptOverridePath returns the override location for a template name.
func PromptOverridePath(name string) string {
	return filepath.Join(promptOverrideDir, name)
}

// DefaultPromptTemplateInfo lists the bundled prompt templates.
func DefaultPromptTemplateInfo() ([]PromptTemplateInfo, error) {
	variables := promptTemplateVariables()
	names := []string{
		"prompt-implementation.tmpl",
		"prompt-feedback.tmpl",
		"prompt-commit-review.tmpl",
		"prompt-project-review.tmpl",
	}
	info := make([]PromptTemplateInfo, 0, len(names))
	for _, name := range names {
		contents, err := readDefaultPromptTemplate(name)
		if err != nil {
			return nil, err
		}
		info = append(info, PromptTemplateInfo{
			Name:      name,
			Contents:  contents,
			Variables: variables,
		})
	}
	return info, nil
}

func readDefaultPromptTemplate(name string) (string, error) {
	data, err := defaultTemplates.ReadFile(filepath.Join("templates", name))
	if err != nil {
		return "", fmt.Errorf("read default prompt %q: %w", name, err)
	}
	return string(data), nil
}

func promptTemplateVariables() []PromptTemplateVariable {
	return []PromptTemplateVariable{
		{Name: "Todo", Type: "todo.Todo"},
		{Name: "Feedback", Type: "string"},
		{Name: "Message", Type: "string"},
		{Name: "AgentTranscripts", Type: "[]AgentTranscript"},
		{Name: "WorkspacePath", Type: "string"},
		{Name: "ReviewInstructions", Type: "string"},
		{Name: "TodoBlock", Type: "string"},
		{Name: "FeedbackBlock", Type: "string"},
		{Name: "CommitMessageBlock", Type: "string"},
		{Name: "SeriesLogBlock", Type: "string"},
		{Name: "TestCommandsBlock", Type: "string"},
	}
}
