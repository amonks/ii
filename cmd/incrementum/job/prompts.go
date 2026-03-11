package job

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	internalagent "monks.co/pkg/agent"
	"monks.co/incrementum/internal/paths"
	internalstrings "monks.co/incrementum/internal/strings"
	"monks.co/incrementum/todo"
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
// ContextFiles are loaded for system prompt assembly and are not used directly in templates.
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

	// Project context templates and files.
	WorkflowContext string
	ReviewQuestions string
	ContextFiles    []string
}

func newPromptData(item todo.Todo, feedback, message, seriesLog string, transcripts []AgentTranscript, workspacePath string, context PromptContext) PromptData {
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
		WorkflowContext:    context.WorkflowContext,
		ReviewQuestions:    context.ReviewQuestions,
		ContextFiles:       context.ContextFiles,
	}
}

// newHabitPromptData creates prompt data for a habit run.
func newHabitPromptData(habitName, habitInstructions, feedback, message string, transcripts []AgentTranscript, workspacePath string, context PromptContext) PromptData {
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
		WorkflowContext:    context.WorkflowContext,
		ReviewQuestions:    context.ReviewQuestions,
		ContextFiles:       context.ContextFiles,
	}
}

// PromptContext holds rendered shared templates and context files.
type PromptContext struct {
	WorkflowContext string
	ReviewQuestions string
	ContextFiles    []string
}

type PromptParts struct {
	ProjectContext []string
	ContextFiles   []string
	PhaseContent   string
	UserContent    string
}

func toPromptContent(phase, userContent string, context PromptContext, testCommands []string) internalagent.PromptContent {
	content := internalagent.PromptContent{
		ProjectContext: filterBlank([]string{context.WorkflowContext, context.ReviewQuestions, reviewInstructionsText}),
		ContextFiles:   context.ContextFiles,
		PhaseContent:   phase,
		UserContent:    userContent,
	}
	if len(testCommands) > 0 {
		content.TestCommands = testCommands
	}
	return content
}

func buildPromptParts(item todo.Todo, feedback, message, seriesLog string, transcripts []AgentTranscript, workspacePath string, testCommands []string, context PromptContext, phaseContent string) (PromptParts, error) {
	data := newPromptData(item, feedback, message, seriesLog, transcripts, workspacePath, context)
	phase, err := RenderPrompt(workspacePath, phaseContent, data)
	if err != nil {
		return PromptParts{}, err
	}

	userTemplate := "{{.TodoBlock}}"
	if data.Feedback != "" {
		userTemplate += "\n\n{{.FeedbackBlock}}"
		if data.Message != "" {
			userTemplate += "\n\nDraft change description (update to reflect your changes):\n{{.CommitMessageBlock}}"
		}
	} else if data.Message != "" {
		userTemplate += "\n\n{{.CommitMessageBlock}}"
	}
	if data.SeriesLogBlock != "" {
		userTemplate += "\n\n{{.SeriesLogBlock}}"
	}
	userTemplate = strings.TrimSpace(userTemplate)

	userContent, err := RenderPrompt(workspacePath, userTemplate, data)
	if err != nil {
		return PromptParts{}, err
	}

	return PromptParts{
		ProjectContext: filterBlank([]string{context.WorkflowContext, context.ReviewQuestions, reviewInstructionsText}),
		ContextFiles:   context.ContextFiles,
		PhaseContent:   phase,
		UserContent:    userContent,
	}, nil
}

func buildHabitPromptParts(habitName, habitInstructions, feedback, message string, transcripts []AgentTranscript, workspacePath string, context PromptContext, phaseContent string) (PromptParts, error) {
	data := newHabitPromptData(habitName, habitInstructions, feedback, message, transcripts, workspacePath, context)
	phase, err := RenderPrompt(workspacePath, phaseContent, data)
	if err != nil {
		return PromptParts{}, err
	}

	userTemplate := ""
	if data.Feedback != "" {
		userTemplate = "{{.FeedbackBlock}}"
		if data.Message != "" {
			userTemplate += "\n\nDraft change description (update to reflect your changes):\n{{.CommitMessageBlock}}"
		}
	} else if data.Message != "" {
		userTemplate = "{{.CommitMessageBlock}}"
	}
	userTemplate = strings.TrimSpace(userTemplate)

	userContent, err := RenderPrompt(workspacePath, userTemplate, data)
	if err != nil {
		return PromptParts{}, err
	}

	return PromptParts{
		ProjectContext: filterBlank([]string{context.WorkflowContext, context.ReviewQuestions, reviewInstructionsText}),
		ContextFiles:   context.ContextFiles,
		PhaseContent:   phase,
		UserContent:    userContent,
	}, nil
}

func promptContentFromParts(parts PromptParts) internalagent.PromptContent {
	return internalagent.PromptContent{
		ProjectContext: parts.ProjectContext,
		ContextFiles:   parts.ContextFiles,
		PhaseContent:   parts.PhaseContent,
		UserContent:    parts.UserContent,
	}
}

func buildPromptContent(phase, userContent, workDir string, opts RunOptions) internalagent.PromptContent {
	context, err := loadPromptContext(workDir)
	if err != nil {
		return internalagent.PromptContent{PhaseContent: phase, UserContent: userContent}
	}
	projectContext := []string{context.WorkflowContext, context.ReviewQuestions, reviewInstructionsText}
	projectContext = filterBlank(projectContext)
	content := internalagent.PromptContent{
		ProjectContext: projectContext,
		ContextFiles:   context.ContextFiles,
		PhaseContent:   phase,
		UserContent:    userContent,
	}
	testCommands := []string{}
	if opts.Config != nil {
		testCommands = opts.Config.Job.TestCommands
	} else if opts.LoadConfig != nil {
		if cfg, cfgErr := opts.LoadConfig(workDir); cfgErr == nil && cfg != nil {
			testCommands = cfg.Job.TestCommands
		}
	}
	if len(testCommands) > 0 {
		content.TestCommands = testCommands
	}
	return content
}

func filterBlank(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if internalstrings.IsBlank(part) {
			continue
		}
		out = append(out, part)
	}
	return out
}

// loadPromptContext loads project-level shared templates and context files.
func loadPromptContext(workspacePath string) (PromptContext, error) {
	workflowTemplate, err := LoadPrompt(workspacePath, workflowContextTemplateName)
	if err != nil {
		return PromptContext{}, err
	}
	workflow, err := renderContextTemplate("workflow_context", workflowTemplate)
	if err != nil {
		return PromptContext{}, err
	}
	reviewTemplate, err := LoadPrompt(workspacePath, reviewQuestionsTemplateName)
	if err != nil {
		return PromptContext{}, err
	}
	reviewQuestions, err := renderContextTemplate("review_questions", reviewTemplate)
	if err != nil {
		return PromptContext{}, err
	}
	contextFiles, err := loadContextFiles(workspacePath)
	if err != nil {
		return PromptContext{}, err
	}
	return PromptContext{
		WorkflowContext: "\n" + workflow + "\n",
		ReviewQuestions: reviewQuestions + "\n",
		ContextFiles:    contextFiles,
	}, nil
}

func loadContextFiles(workspacePath string) ([]string, error) {
	globalConfigDir, err := paths.DefaultConfigDir()
	if err != nil {
		return nil, err
	}
	files, err := internalagent.LoadContextFiles(internalagent.LoadContextFilesOptions{WorkDir: workspacePath, GlobalConfigDir: globalConfigDir})
	if err != nil {
		return nil, err
	}
	var contents []string
	for _, f := range files {
		trimmed := internalstrings.TrimTrailingNewlines(f.Content)
		if internalstrings.IsBlank(trimmed) {
			continue
		}
		contents = append(contents, trimmed)
	}
	return contents, nil
}

func renderContextTemplate(name, contents string) (string, error) {
	tmpl, err := template.New("context").Option("missingkey=error").Parse(contents)
	if err != nil {
		return "", fmt.Errorf("parse %s template: %w", name, err)
	}
	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, name, nil); err != nil {
		return "", fmt.Errorf("render %s template: %w", name, err)
	}
	return strings.TrimSpace(out.String()), nil
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
	var tmpl *template.Template
	var err error
	tmpl = template.New("prompt").Option("missingkey=error")
	if tmplText := contextTemplate("workflow_context", data.WorkflowContext); tmplText != "" {
		tmpl, err = tmpl.Parse(tmplText)
		if err != nil {
			return "", fmt.Errorf("parse workflow context template: %w", err)
		}
	}
	if tmplText := contextTemplate("review_questions", data.ReviewQuestions); tmplText != "" {
		tmpl, err = tmpl.Parse(tmplText)
		if err != nil {
			return "", fmt.Errorf("parse review questions template: %w", err)
		}
	}

	tmpl, err = tmpl.Parse(contents)
	if err != nil {
		return "", fmt.Errorf("parse prompt: %w", err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render prompt: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}

func trimmedPromptOutputNoTrailingSpaces(value string) string {
	if internalstrings.IsBlank(value) {
		return ""
	}
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}

func contextTemplate(name, contents string) string {
	if internalstrings.IsBlank(contents) {
		return ""
	}
	return fmt.Sprintf("{{define %q}}%s{{end}}", name, contents)
}
