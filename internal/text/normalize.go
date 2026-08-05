// Package text holds the string helpers that ii uses in more than one place:
// canonicalization of user-supplied tokens, newline and whitespace trimming,
// and terminal block indentation.
//
// It deliberately does not wrap stdlib strings functions. A helper earns a
// place here only when it names a concept the stdlib call does not — "blank",
// "newlines", "indent a block" — so that call sites read as intent rather than
// as an extra hop.
package text

import (
	"strings"
	"unicode"
)

// NormalizeWhitespace collapses runs of whitespace into single spaces.
func NormalizeWhitespace(value string) string {
	fields := strings.Fields(value)
	return strings.Join(fields, " ")
}

// NormalizeLowerTrimSpace trims surrounding whitespace and lowercases the
// input. This is the canonical form for user-typed enum tokens such as todo
// statuses and types.
func NormalizeLowerTrimSpace(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// IsBlank reports whether the string contains only whitespace.
func IsBlank(value string) bool {
	return strings.TrimSpace(value) == ""
}

// ContainsAnyLower reports whether lowercased value contains any substrings.
// Substrings should be provided in lowercase.
func ContainsAnyLower(value string, substrings ...string) bool {
	if len(substrings) == 0 {
		return false
	}
	value = strings.ToLower(value)
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

// TrimTrailingNewlines removes trailing CR/LF characters.
func TrimTrailingNewlines(value string) string {
	return strings.TrimRight(value, "\r\n")
}

// TrimLeadingNewlines removes leading CR/LF characters.
func TrimLeadingNewlines(value string) string {
	return strings.TrimLeft(value, "\r\n")
}

// TrimTrailingWhitespace removes trailing Unicode whitespace characters,
// leaving any leading indentation intact.
func TrimTrailingWhitespace(value string) string {
	return strings.TrimRightFunc(value, unicode.IsSpace)
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
