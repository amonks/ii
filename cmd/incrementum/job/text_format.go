package job

import (
	"strings"

	"github.com/amonks/incrementum/internal/markdown"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/muesli/reflow/wordwrap"
)

const (
	lineWidth         = 80
	documentIndent    = 4
	subdocumentIndent = 8
)

func wrapWidthFor(width int, indent int) int {
	width -= indent
	if width < 1 {
		return 1
	}
	return width
}

// RenderMarkdown formats markdown text for terminal display.
func RenderMarkdown(value string, width int) string {
	rendered := markdown.SafeRender(width, 0, []byte(value))
	return string(rendered)
}

// ReflowParagraphs wraps and normalizes paragraph text.
func ReflowParagraphs(value string, width int) string {
	paragraphs := splitParagraphs(value)
	wrapped := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		normalized := internalstrings.NormalizeWhitespace(paragraph)
		if normalized == "" {
			continue
		}
		wrapped = append(wrapped, wordwrap.String(normalized, width))
	}
	return strings.Join(wrapped, "\n\n")
}

func splitParagraphs(value string) []string {
	lines := strings.Split(value, "\n")
	var paragraphs []string
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		paragraphs = append(paragraphs, strings.Join(current, " "))
		current = nil
	}
	for _, line := range lines {
		if internalstrings.IsBlank(line) {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()
	return paragraphs
}

// IndentBlock prefixes each line with spaces.
func IndentBlock(value string, spaces int) string {
	value = internalstrings.TrimTrailingNewlines(value)
	return internalstrings.IndentBlock(value, spaces)
}

// ReflowIndentedText wraps and preserves indentation levels.
func ReflowIndentedText(value string, width int, baseIndent int) string {
	value = internalstrings.NormalizeNewlines(value)
	value = internalstrings.TrimTrailingNewlines(value)
	if internalstrings.IsBlank(value) {
		return IndentBlock("-", baseIndent)
	}

	lines := strings.Split(value, "\n")
	var out []string
	for i := 0; i < len(lines); {
		line := lines[i]
		if internalstrings.IsBlank(line) {
			out = append(out, strings.Repeat(" ", baseIndent))
			i++
			continue
		}
		indent := internalstrings.LeadingSpaces(line)
		var parts []string
		for i < len(lines) {
			line = lines[i]
			if internalstrings.IsBlank(line) {
				break
			}
			if internalstrings.LeadingSpaces(line) != indent {
				break
			}
			parts = append(parts, internalstrings.TrimSpace(line[indent:]))
			i++
		}
		normalized := internalstrings.NormalizeWhitespace(strings.Join(parts, " "))
		if normalized == "" {
			out = append(out, strings.Repeat(" ", baseIndent+indent)+"-")
			continue
		}
		wrapped := wordwrap.String(normalized, wrapWidthFor(width, baseIndent+indent))
		wrapped = IndentBlock(wrapped, baseIndent+indent)
		out = append(out, strings.Split(wrapped, "\n")...)
	}
	return strings.Join(out, "\n")
}
