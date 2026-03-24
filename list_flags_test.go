package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestListCommandsHaveAllFlag(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "todo", cmd: todoListCmd},
		{name: "workspace", cmd: workspaceListCmd},
		{name: "agent", cmd: agentListCmd},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flag := tc.cmd.Flags().Lookup("all")
			if flag == nil {
				t.Fatalf("expected --all flag for %s list", tc.name)
			}
			if flag.DefValue != "false" {
				t.Fatalf("expected default --all false for %s, got %q", tc.name, flag.DefValue)
			}
		})
	}
}
