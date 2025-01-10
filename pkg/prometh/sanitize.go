package prometh

import "unicode"

func SanitizeLabels(ss ...string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = SanitizeLabel(s)
	}
	return out
}

// SanitizeLabel converts a string into a valid Prometheus label name by replacing
// invalid characters with underscores and ensuring it starts with a letter or underscore.
// Valid characters are ASCII letters, numbers, and underscores.
func SanitizeLabel(s string) string {
	if s == "" {
		return "_"
	}

	// Convert to runes for proper character handling
	runes := []rune(s)
	result := make([]rune, len(runes))

	// First character must be a letter or underscore
	if !isValidFirstChar(runes[0]) {
		result[0] = '_'
	} else {
		result[0] = runes[0]
	}

	// Process remaining characters
	for i := 1; i < len(runes); i++ {
		if isValidChar(runes[i]) {
			result[i] = runes[i]
		} else {
			result[i] = '_'
		}
	}

	return string(result)
}

// isValidFirstChar returns true if the rune is valid as the first character
// of a Prometheus label (ASCII letter or underscore)
func isValidFirstChar(r rune) bool {
	return r == '_' || (r <= unicode.MaxASCII && unicode.IsLetter(r))
}

// isValidChar returns true if the rune is valid for a Prometheus label
// (ASCII letter, number, or underscore)
func isValidChar(r rune) bool {
	return r == '_' || (r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)))
}
