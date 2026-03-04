package main

import (
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	jobpkg "github.com/amonks/incrementum/job"
)

func renderMarkdownOrDash(value string, width int) string {
	formatted := jobpkg.RenderMarkdown(value, width)
	if internalstrings.IsBlank(formatted) {
		return "-"
	}
	return formatted
}

func renderMarkdownWithoutMargin(value string, width int) string {
	formatted := renderMarkdownOrDash(value, width)
	return trimCommonIndent(formatted)
}

func trimCommonIndent(value string) string {
	lines := strings.Split(value, "\n")
	minIndent := -1
	for _, line := range lines {
		if internalstrings.IsBlank(line) {
			continue
		}
		indent := internalstrings.LeadingSpaces(line)
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
		if minIndent == 0 {
			break
		}
	}
	if minIndent <= 0 {
		return value
	}
	indentStr := strings.Repeat(" ", minIndent)
	for i, line := range lines {
		if internalstrings.IsBlank(line) {
			lines[i] = ""
			continue
		}
		lines[i] = strings.TrimPrefix(line, indentStr)
	}
	return strings.Join(lines, "\n")
}
