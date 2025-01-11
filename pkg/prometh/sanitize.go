package prometh

import "unicode"

func SanitizeLabels(ss ...string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = SanitizeLabel(s)
	}
	return out
}

// SanitizeLabel converts a string into a valid Prometheus label name by
// replacing invalid characters with underscores. Valid characters are ASCII
// letters, numbers, and underscores.
func SanitizeLabel(s string) string {
	if s == "" {
		return "_"
	}

	// Convert to runes for proper character handling
	runes := []rune(s)
	result := make([]rune, len(runes))

	// Process remaining characters
	for i, r := range runes {
		if isValidChar(r) {
			result[i] = r
		} else {
			result[i] = '_'
		}
	}

	return string(result)
}

// isValidChar returns true if the rune is valid for a Prometheus label
// (ASCII letter, number, or underscore)
func isValidChar(r rune) bool {
	return r == '_' || (r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)))
}
