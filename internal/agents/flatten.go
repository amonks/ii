package agents

import (
	"fmt"
	"strings"

	"monks.co/pkg/agent"
)

// FlattenPrompt converts structured PromptContent into a flat markdown string
// suitable for CLI agents that accept a single text prompt.
func FlattenPrompt(p agent.PromptContent) string {
	var sections []string

	if len(p.ProjectContext) > 0 {
		joined := strings.Join(p.ProjectContext, "\n\n")
		if strings.TrimSpace(joined) != "" {
			sections = append(sections, "# Project Context\n\n"+joined)
		}
	}

	if len(p.ContextFiles) > 0 {
		joined := strings.Join(p.ContextFiles, "\n\n---\n\n")
		if strings.TrimSpace(joined) != "" {
			sections = append(sections, "# Context Files\n\n"+joined)
		}
	}

	if len(p.TestCommands) > 0 {
		var items []string
		for _, cmd := range p.TestCommands {
			cmd = strings.TrimSpace(cmd)
			if cmd != "" {
				items = append(items, fmt.Sprintf("- `%s`", cmd))
			}
		}
		if len(items) > 0 {
			sections = append(sections, "# Test Commands\n\n"+strings.Join(items, "\n"))
		}
	}

	if strings.TrimSpace(p.PhaseContent) != "" {
		sections = append(sections, "# Phase Instructions\n\n"+p.PhaseContent)
	}

	if strings.TrimSpace(p.UserContent) != "" {
		sections = append(sections, "# Task\n\n"+p.UserContent)
	}

	return strings.Join(sections, "\n\n")
}
