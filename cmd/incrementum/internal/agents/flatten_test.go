package agents

import (
	"strings"
	"testing"

	"monks.co/pkg/agent"
)

func TestFlattenPrompt_AllSections(t *testing.T) {
	p := agent.PromptContent{
		ProjectContext: []string{"context1", "context2"},
		ContextFiles:   []string{"file1 content", "file2 content"},
		TestCommands:   []string{"go test ./...", "golangci-lint run"},
		PhaseContent:   "Implement the feature",
		UserContent:    "Add a login button",
	}

	result := FlattenPrompt(p)

	if !strings.Contains(result, "# Project Context") {
		t.Error("expected Project Context heading")
	}
	if !strings.Contains(result, "context1") {
		t.Error("expected context1")
	}
	if !strings.Contains(result, "# Context Files") {
		t.Error("expected Context Files heading")
	}
	if !strings.Contains(result, "# Test Commands") {
		t.Error("expected Test Commands heading")
	}
	if !strings.Contains(result, "`go test ./...`") {
		t.Error("expected test command")
	}
	if !strings.Contains(result, "# Phase Instructions") {
		t.Error("expected Phase Instructions heading")
	}
	if !strings.Contains(result, "# Task") {
		t.Error("expected Task heading")
	}
	if !strings.Contains(result, "Add a login button") {
		t.Error("expected user content")
	}
}

func TestFlattenPrompt_EmptySections(t *testing.T) {
	p := agent.PromptContent{
		UserContent: "Just do it",
	}

	result := FlattenPrompt(p)

	if strings.Contains(result, "# Project Context") {
		t.Error("should not have Project Context when empty")
	}
	if !strings.Contains(result, "# Task") {
		t.Error("expected Task heading")
	}
}
