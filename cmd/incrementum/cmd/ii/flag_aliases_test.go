package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDescriptionAliasUsesSingleFlag(t *testing.T) {
	var description string
	cmd := &cobra.Command{Use: "example"}
	addDescriptionFlagAliases(cmd)
	cmd.Flags().StringVarP(&description, "description", "d", "", "Example description")

	if err := cmd.Flags().Set("desc", "Hello"); err != nil {
		t.Fatalf("set desc alias: %v", err)
	}
	if description != "Hello" {
		t.Fatalf("expected description to be set via alias, got %q", description)
	}
	if !cmd.Flags().Changed("description") {
		t.Fatal("expected description flag to be marked as changed")
	}

	usage := cmd.Flags().FlagUsages()
	if strings.Contains(usage, "--desc ") {
		t.Fatalf("did not expect alias to appear in usage, got %q", usage)
	}
	if !strings.Contains(usage, "-d, --description") {
		t.Fatalf("expected shorthand to appear inline, got %q", usage)
	}
}
