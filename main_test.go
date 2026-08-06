package main

import (
	"slices"
	"testing"
)

func TestRootCommandName(t *testing.T) {
	if rootCmd.Use != "ii" {
		t.Fatalf("expected root command name ii, got %q", rootCmd.Use)
	}
}

// TestRootCommandsAreTodoCommands pins the CLI surface: ii is a todo manager,
// so every todo verb is a root subcommand and nothing else is registered.
// `help` is absent because it is attached via SetHelpCommand at execute time.
func TestRootCommandsAreTodoCommands(t *testing.T) {
	want := []string{
		"close", "create", "delete", "dep", "finish", "list",
		"ready", "reopen", "show", "start", "update",
	}

	var got []string
	for _, cmd := range rootCmd.Commands() {
		got = append(got, cmd.Name())
	}
	slices.Sort(got)

	if !slices.Equal(got, want) {
		t.Fatalf("root subcommands: got %v, want %v", got, want)
	}
}
