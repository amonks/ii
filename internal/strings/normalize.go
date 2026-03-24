package strings

import (
	"strings"
	"unicode"
)

// NormalizeWhitespace collapses runs of whitespace into single spaces.
func NormalizeWhitespace(value string) string {
	fields := strings.Fields(value)
	return strings.Join(fields, " ")
}

// NormalizeLower returns the input lowercased.
func NormalizeLower(value string) string {
	return strings.ToLower(value)
}

// NormalizeLowerTrimSpace trims surrounding whitespace and lowercases the input.
func NormalizeLowerTrimSpace(value string) string {
	return NormalizeLower(TrimSpace(value))
}

// TrimSpace trims surrounding whitespace.
func TrimSpace(value string) string {
	return strings.TrimSpace(value)
}

// IsBlank reports whether the string contains only whitespace.
func IsBlank(value string) bool {
	return TrimSpace(value) == ""
}

// ContainsAnyLower reports whether lowercased value contains any substrings.
// Substrings should be provided in lowercase.
func ContainsAnyLower(value string, substrings ...string) bool {
	if len(substrings) == 0 {
		return false
	}
	value = NormalizeLower(value)
	for _, substring := range substrings {
		if strings.Contains(value, substring) {
			return true
		}
	}
	return false
}

// NormalizeNewlines replaces CRLF and CR with LF.
func NormalizeNewlines(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

// TrimTrailingCarriageReturn removes a trailing carriage return if present.
func TrimTrailingCarriageReturn(value string) string {
	return strings.TrimSuffix(value, "\r")
}

// TrimTrailingNewlines removes trailing CR/LF characters.
func TrimTrailingNewlines(value string) string {
	return strings.TrimRight(value, "\r\n")
}

// TrimLeadingNewlines removes leading CR/LF characters.
func TrimLeadingNewlines(value string) string {
	return strings.TrimLeft(value, "\r\n")
}

// TrimTrailingWhitespace removes trailing Unicode whitespace characters.
func TrimTrailingWhitespace(value string) string {
	return strings.TrimRightFunc(value, unicode.IsSpace)
}

// TrimTrailingSlash removes trailing '/' characters.
func TrimTrailingSlash(value string) string {
	return strings.TrimRight(value, "/")
}

// LeadingSpaces counts leading ASCII space characters.
func LeadingSpaces(value string) int {
	count := 0
	for _, char := range value {
		if char != ' ' {
			break
		}
		count++
	}
	return count
}

// IndentBlock prefixes each line with spaces.
func IndentBlock(value string, spaces int) string {
	if spaces <= 0 {
		return value
	}
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
