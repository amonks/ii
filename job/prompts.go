package job

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/todo"
)

const (
	promptOverrideDir              = ".incrementum/templates"
	reviewQuestionsTemplateName    = "review-questions.tmpl"
	reviewInstructionsTemplateName = "review-instructions.tmpl"
	workflowContextTemplateName    = "workflow-context.tmpl"
)

//go:embed templates/*.tmpl
var defaultTemplates embed.FS

var reviewInstructionsText = mustReadDefaultPromptTemplate(reviewInstructionsTemplateName)

// PromptData supplies values for job prompt templates.
type PromptData struct {
	Todo               todo.Todo
	Feedback           string
	Message            string
	AgentTranscripts   []AgentTranscript
	WorkspacePath      string
	ReviewInstructions string
	TodoBlock          string
	FeedbackBlock      string
	CommitMessageBlock string
	SeriesLogBlock     string

	// Habit fields (empty for regular todo jobs)
	HabitName         string
	HabitInstructions string
}

func newPromptData(item todo.Todo, feedback, message, seriesLog string, transcripts []AgentTranscript, workspacePath string) PromptData {
	return PromptData{
		Todo:               item,
		Feedback:           feedback,
		Message:            message,
		AgentTranscripts:   transcripts,
		WorkspacePath:      workspacePath,
		ReviewInstructions: reviewInstructionsText,
		TodoBlock:          formatTodoBlock(item),
		FeedbackBlock:      formatFeedbackBlock(feedback),
		CommitMessageBlock: formatPromptBlock("Change description", message),
		SeriesLogBlock:     formatSeriesLogBlock(seriesLog),
	}
}

// newHabitPromptData creates prompt data for a habit run.
func newHabitPromptData(habitName, habitInstructions, feedback, message string, transcripts []AgentTranscript, workspacePath string) PromptData {
	return PromptData{
		Feedback:           feedback,
		Message:            message,
		AgentTranscripts:   transcripts,
		WorkspacePath:      workspacePath,
		ReviewInstructions: reviewInstructionsText,
		FeedbackBlock:      formatFeedbackBlock(feedback),
		CommitMessageBlock: formatPromptBlock("Change description", message),
		HabitName:          habitName,
		HabitInstructions:  formatHabitInstructions(habitInstructions),
	}
}

func formatHabitInstructions(instructions string) string {
	instructions = internalstrings.TrimTrailingNewlines(instructions)
	if internalstrings.IsBlank(instructions) {
		return "-"
	}
	return IndentBlock(instructions, documentIndent)
}

func mustReadDefaultPromptTemplate(name string) string {
	contents, err := readDefaultPromptTemplate(name)
	if err != nil {
		panic(fmt.Sprintf("load default prompt template %q: %v", name, err))
	}
	return contents
}

func formatTodoBlock(item todo.Todo) string {
	description := internalstrings.TrimTrailingNewlines(item.Description)
	if internalstrings.IsBlank(description) {
		description = "-"
	}
	description = ReflowIndentedText(description, lineWidth, subdocumentIndent)
	fields := []string{
		formatTodoField("ID", item.ID),
		formatTodoField("Title", item.Title),
		formatTodoField("Type", string(item.Type)),
		formatTodoField("Priority", fmt.Sprintf("%d", item.Priority)),
		"Description:",
	}
	fieldBlock := IndentBlock(strings.Join(fields, "\n"), documentIndent)
	return fmt.Sprintf("Todo\n\n%s\n%s", fieldBlock, description)
}

func formatPromptBlock(label, body string) string {
	body = internalstrings.TrimTrailingNewlines(body)
	if internalstrings.IsBlank(body) {
		body = "-"
	}
	formatted := ReflowIndentedText(body, lineWidth, documentIndent)
	return fmt.Sprintf("%s\n\n%s", label, formatted)
}

func formatFeedbackBlock(body string) string {
	if looksLikeMarkdownList(body) {
		return formatPromptMarkdownBlock("Previous feedback", body)
	}
	return formatPromptBlock("Previous feedback", body)
}

func formatSeriesLogBlock(seriesLog string) string {
	seriesLog = internalstrings.TrimTrailingNewlines(seriesLog)
	if internalstrings.IsBlank(seriesLog) {
		return ""
	}
	return fmt.Sprintf("Series so far (commits in this patch series):\n\n```\n%s\n```", seriesLog)
}

func formatPromptMarkdownBlock(label, body string) string {
	body = internalstrings.TrimTrailingNewlines(body)
	if internalstrings.IsBlank(body) {
		body = "-"
	}
	formatted := RenderMarkdown(body, lineWidth)
	formatted = normalizePromptMarkdown(formatted)
	if internalstrings.IsBlank(formatted) {
		formatted = "-"
	}
	formatted = IndentBlock(formatted, documentIndent)
	return fmt.Sprintf("%s\n\n%s", label, formatted)
}

func looksLikeMarkdownList(body string) bool {
	body = internalstrings.NormalizeNewlines(body)
	trimmed := internalstrings.TrimSpace(body)
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return true
	}
	return strings.Contains(body, "\n- ") || strings.Contains(body, "\n* ")
}

func normalizePromptMarkdown(value string) string {
	value = internalstrings.NormalizeNewlines(value)
	value = internalstrings.TrimTrailingNewlines(value)
	value = promptTrimLeadingBlankLines(value)
	if internalstrings.IsBlank(value) {
		return ""
	}
	if !looksLikeMarkdownListBlock(value) {
		return value
	}
	return promptTrimCommonIndent(value)
}

func looksLikeMarkdownListBlock(value string) bool {
	lines := strings.SplitSeq(value, "\n")
	for line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			return true
		}
	}
	return false
}

func promptTrimLeadingBlankLines(value string) string {
	lines := strings.Split(value, "\n")
	start := 0
	for start < len(lines) {
		if !internalstrings.IsBlank(lines[start]) {
			break
		}
		start++
	}
	if start >= len(lines) {
		return ""
	}
	return strings.Join(lines[start:], "\n")
}

func promptTrimCommonIndent(value string) string {
	lines := strings.Split(value, "\n")
	minIndent := -1
	for _, line := range lines {
		if internalstrings.IsBlank(line) {
			continue
		}
		spaces := internalstrings.LeadingSpaces(line)
		if minIndent == -1 || spaces < minIndent {
			minIndent = spaces
		}
	}
	if minIndent <= 0 {
		return value
	}
	for i, line := range lines {
		if internalstrings.IsBlank(line) {
			lines[i] = ""
			continue
		}
		if len(line) <= minIndent {
			lines[i] = ""
			continue
		}
		lines[i] = line[minIndent:]
	}
	return strings.Join(lines, "\n")
}

func formatTodoField(label, value string) string {
	value = internalstrings.NormalizeWhitespace(value)
	if value == "" {
		value = "-"
	}
	return fmt.Sprintf("%s: %s", label, value)
}

// LoadPrompt loads a prompt template for the repo.
func LoadPrompt(repoPath, name string) (string, error) {
	if internalstrings.IsBlank(name) {
		return "", fmt.Errorf("prompt name is required")
	}

	if repoPath != "" {
		overridePath := filepath.Join(repoPath, promptOverrideDir, name)
		if data, err := os.ReadFile(overridePath); err == nil {
			return string(data), nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("read prompt override: %w", err)
		}
	}

	data, err := defaultTemplates.ReadFile(filepath.Join("templates", name))
	if err != nil {
		return "", fmt.Errorf("read default prompt: %w", err)
	}

	return string(data), nil
}

// RenderPrompt renders the prompt with provided data.
func RenderPrompt(repoPath, contents string, data PromptData) (string, error) {
	reviewQuestionsTemplate, err := LoadPrompt(repoPath, reviewQuestionsTemplateName)
	if err != nil {
		return "", fmt.Errorf("load review questions template: %w", err)
	}

	workflowContextTemplate, err := LoadPrompt(repoPath, workflowContextTemplateName)
	if err != nil {
		return "", fmt.Errorf("load workflow context template: %w", err)
	}

	tmpl, err := template.New("prompt").Option("missingkey=error").Parse(reviewQuestionsTemplate)
	if err != nil {
		return "", fmt.Errorf("parse review questions template: %w", err)
	}

	tmpl, err = tmpl.Parse(workflowContextTemplate)
	if err != nil {
		return "", fmt.Errorf("parse workflow context template: %w", err)
	}

	tmpl, err = tmpl.Parse(contents)
	if err != nil {
		return "", fmt.Errorf("parse prompt: %w", err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render prompt: %w", err)
	}
	return out.String(), nil
}
