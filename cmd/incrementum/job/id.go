package job

import (
	"time"

	"github.com/amonks/incrementum/internal/ids"
)

// GenerateID creates a unique ID from a todo ID and timestamp.
func GenerateID(todoID string, timestamp time.Time) string {
	return ids.GenerateWithTimestamp(todoID, timestamp, ids.DefaultLength)
}
