package validation

import (
	"fmt"
	"slices"
	"strings"
)

// IsValidValue returns true when a value is in the valid list.
func IsValidValue[T comparable](value T, valid []T) bool {
	return slices.Contains(valid, value)
}

// FormatValidValues joins string-like values for error messages.
func FormatValidValues[T ~string](values []T) string {
	formatted := make([]string, 0, len(values))
	for _, value := range values {
		formatted = append(formatted, string(value))
	}
	return strings.Join(formatted, ", ")
}

// FormatInvalidValueError builds a consistent invalid-value error.
func FormatInvalidValueError[T ~string](err error, value T, valid []T) error {
	return fmt.Errorf("%w: %q (valid: %s)", err, value, FormatValidValues(valid))
}
