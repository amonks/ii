package job

import (
	"fmt"
	"strings"
)

// TestCommandResult captures a test command execution result.
type TestCommandResult struct {
	Command  string
	ExitCode int
	Output   string
}

// FormatTestFeedback builds a markdown list describing test outcomes.
func FormatTestFeedback(results []TestCommandResult) string {
	if len(results) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, result := range results {
		status := "passing"
		if result.ExitCode != 0 {
			status = "failing"
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		fmt.Fprintf(&builder, "- %s is %s", result.Command, status)
	}

	return builder.String()
}
