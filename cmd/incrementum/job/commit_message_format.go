package job

import (
	"fmt"
	"strings"

	internalstrings "monks.co/incrementum/internal/strings"
	"monks.co/incrementum/todo"
)

func formatCommitMessage(item todo.Todo, message, reviewComments string) string {
	return formatCommitMessageWithWidth(item, message, reviewComments, lineWidth)
}

func formatCommitMessageWithWidth(item todo.Todo, message, reviewComments string, width int) string {
	summary, body := splitCommitMessage(message)
	formatted := renderMarkdownText(summary, width)

	bodyText := renderMarkdownTextOrDash(body, width-documentIndent)
	formatted += "\n\nHere is a generated change description:\n\n"
	formatted += IndentBlock(bodyText, documentIndent)

	if reviewComments != "" {
		reviewText := renderMarkdownText(reviewComments, width-documentIndent)
		formatted += "\n\nReview comments:\n\n"
		formatted += IndentBlock(reviewText, documentIndent)
	}

	formatted += "\n\nThis change is part of a series implementing this todo:\n\n"
	formatted += formatCommitTodoWithWidth(item, width)
	return normalizeFormattedCommitMessage(formatted)
}

func splitCommitMessage(message string) (string, string) {
	message = normalizeCommitMessage(message)
	lines := strings.Split(message, "\n")
	summary := lines[0]
	body := strings.Join(lines[1:], "\n")
	return summary, body
}

func formatCommitTodoWithWidth(item todo.Todo, width int) string {
	fields := []string{
		fmt.Sprintf("ID: %s", item.ID),
		fmt.Sprintf("Title: %s", item.Title),
		fmt.Sprintf("Type: %s", item.Type),
		fmt.Sprintf("Priority: %d (%s)", item.Priority, todo.PriorityName(item.Priority)),
		"Description:",
	}
	fieldBlock := renderMarkdownLines(fields, width-documentIndent)
	fieldBlock = IndentBlock(fieldBlock, documentIndent)

	description := renderMarkdownTextOrDash(item.Description, width-subdocumentIndent)
	description = IndentBlock(description, subdocumentIndent)
	return fieldBlock + "\n" + description
}

func renderMarkdownText(value string, width int) string {
	value = internalstrings.TrimSpace(value)
	return renderMarkdownTextFromTrimmed(value, width)
}

func renderMarkdownTextFromTrimmed(value string, width int) string {
	if value == "" {
		return ""
	}
	rendered := RenderMarkdown(value, width)
	return internalstrings.TrimTrailingNewlines(rendered)
}

func renderMarkdownTextOrDash(value string, width int) string {
	rendered := renderMarkdownText(value, width)
	if internalstrings.IsBlank(rendered) {
		return "-"
	}
	return rendered
}

func renderMarkdownLines(lines []string, width int) string {
	renderedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		line = internalstrings.TrimSpace(line)
		if line == "" {
			renderedLines = append(renderedLines, "")
			continue
		}
		rendered := renderMarkdownTextFromTrimmed(line, width)
		if internalstrings.IsBlank(rendered) {
			renderedLines = append(renderedLines, "-")
			continue
		}
		renderedLines = append(renderedLines, strings.Split(rendered, "\n")...)
	}
	return strings.Join(renderedLines, "\n")
}
