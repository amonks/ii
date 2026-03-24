package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestHasChangedFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "example"}
	cmd.Flags().String("title", "", "")
	cmd.Flags().String("description", "", "")

	if hasChangedFlags(cmd, "title", "description") {
		t.Fatal("expected no changed flags")
	}

	if err := cmd.Flags().Set("description", "hello"); err != nil {
		t.Fatalf("set description: %v", err)
	}

	if !hasChangedFlags(cmd, "title", "description") {
		t.Fatal("expected changed flags")
	}
}
