package main

import "testing"

func TestRootCommandName(t *testing.T) {
	if rootCmd.Use != "ii" {
		t.Fatalf("expected root command name ii, got %q", rootCmd.Use)
	}
}

func TestRootCommandDoesNotIncludeSession(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "session" {
			t.Fatalf("unexpected session subcommand registered")
		}
	}
}
