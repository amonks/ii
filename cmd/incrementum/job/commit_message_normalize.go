package job

import (
	"strings"
	"unicode"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

func normalizeCommitMessage(message string) string {
	message = internalstrings.NormalizeNewlines(message)
	message = internalstrings.TrimTrailingNewlines(message)
	if message == "" {
		return ""
	}
	lines := strings.Split(message, "\n")
	for i, line := range lines {
		lines[i] = internalstrings.TrimTrailingWhitespace(line)
	}
	message = strings.Join(lines, "\n")
	return trimLeadingBlankLines(message)
}

func normalizeFormattedCommitMessage(message string) string {
	message = normalizeCommitMessage(message)
	if message == "" {
		return ""
	}
	lines := strings.Split(message, "\n")
	for i, line := range lines {
		if internalstrings.IsBlank(line) {
			continue
		}
		lines[i] = strings.TrimLeftFunc(line, unicode.IsSpace)
		break
	}
	return normalizeCommitMessage(strings.Join(lines, "\n"))
}

func trimLeadingBlankLines(message string) string {
	lines := strings.Split(message, "\n")
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
