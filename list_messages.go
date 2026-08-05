package main

import (
	"fmt"
	"strings"

	"monks.co/ii/internal/text"
)

func todoEmptyListMessage(total int, status string, includeAll bool, includeTombstones bool, hasDone bool, hasTombstones bool) string {
	if total == 0 {
		return "No todos found."
	}

	status = text.NormalizeLowerTrimSpace(status)
	if status != "" {
		return fmt.Sprintf("No todos found with status %s.", status)
	}

	hints := make([]string, 0, 2)
	if !includeAll && hasDone {
		hints = append(hints, "Use --all to include done todos.")
	}
	if !includeTombstones && hasTombstones {
		hints = append(hints, "Use --tombstones to include deleted todos.")
	}
	if len(hints) > 0 {
		return fmt.Sprintf("No todos found. %s", strings.Join(hints, " "))
	}

	return "No todos found."
}
