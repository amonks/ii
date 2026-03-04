package main

import (
	"fmt"
	"strings"

	internalstrings "monks.co/incrementum/internal/strings"
)

func jobEmptyListMessage(total int, status string, includeAll bool) string {
	if total == 0 {
		return "No jobs found."
	}

	status = internalstrings.NormalizeLowerTrimSpace(status)
	if status != "" {
		return fmt.Sprintf("No jobs found with status %s.", status)
	}

	if !includeAll {
		return "No active jobs found. Use --all to include completed/failed/abandoned jobs."
	}

	return "No jobs found."
}

func agentEmptyListMessage(total int, includeAll bool) string {
	if total > 0 && !includeAll {
		return "No active agent sessions found. Use --all to include completed/failed sessions."
	}

	return "No agent sessions found."
}

func todoEmptyListMessage(total int, status string, includeAll bool, includeTombstones bool, hasDone bool, hasTombstones bool) string {
	if total == 0 {
		return "No todos found."
	}

	status = internalstrings.NormalizeLowerTrimSpace(status)
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
