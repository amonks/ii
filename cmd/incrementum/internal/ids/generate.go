package ids

import (
	"crypto/sha256"
	"encoding/base32"
	"time"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// DefaultLength is the standard length for generated IDs.
const DefaultLength = 8

// Generate creates a deterministic, lowercase base32 ID derived from input.
func Generate(input string, length int) string {
	hash := sha256.Sum256([]byte(input))
	encoded := base32.StdEncoding.EncodeToString(hash[:])
	if length <= 0 {
		return ""
	}
	if length > len(encoded) {
		length = len(encoded)
	}
	return internalstrings.NormalizeLower(encoded[:length])
}

// GenerateWithTimestamp appends a timestamp to input before hashing.
func GenerateWithTimestamp(input string, timestamp time.Time, length int) string {
	return Generate(input+timestamp.Format(time.RFC3339Nano), length)
}
