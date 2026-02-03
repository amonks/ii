package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestTodoReasonFlagOnlyOnDelete(t *testing.T) {
	flag := todoDeleteCmd.Flags().Lookup("reason")
	if flag == nil {
		t.Fatal("expected todo delete to have --reason flag")
	}
	if flag.DefValue != "" {
		t.Fatalf("expected todo delete reason default empty, got %q", flag.DefValue)
	}

	cases := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "close", cmd: todoCloseCmd},
		{name: "finish", cmd: todoFinishCmd},
		{name: "reopen", cmd: todoReopenCmd},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cmd.Flags().Lookup("reason") != nil {
				t.Fatalf("did not expect --reason flag for todo %s", tc.name)
			}
		})
	}
}

func TestTodoListHasTombstonesFlag(t *testing.T) {
	flag := todoListCmd.Flags().Lookup("tombstones")
	if flag == nil {
		t.Fatal("expected todo list to have --tombstones flag")
	}
	if flag.DefValue != "false" {
		t.Fatalf("expected todo list tombstones default false, got %q", flag.DefValue)
	}
}

func TestTodoUpdateStatusMentionsTombstone(t *testing.T) {
	flag := todoUpdateCmd.Flags().Lookup("status")
	if flag == nil {
		t.Fatal("expected todo update to have --status flag")
	}
	if !strings.Contains(flag.Usage, "proposed") {
		t.Fatalf("expected todo update status usage to mention proposed, got %q", flag.Usage)
	}
	if !strings.Contains(flag.Usage, "tombstone") {
		t.Fatalf("expected todo update status usage to mention tombstone, got %q", flag.Usage)
	}
}

func TestTodoCreateHasTitleFlag(t *testing.T) {
	flag := todoCreateCmd.Flags().Lookup("title")
	if flag == nil {
		t.Fatal("expected todo create to have --title flag")
	}
	if flag.DefValue != "" {
		t.Fatalf("expected todo create title default empty, got %q", flag.DefValue)
	}
}

func TestTodoFinishHasDoneAlias(t *testing.T) {
	if !containsAlias(todoFinishCmd, "done") {
		t.Fatalf("expected todo finish to have done alias, got %v", todoFinishCmd.Aliases)
	}
}

func TestTodoUpdateHasEditAlias(t *testing.T) {
	if !containsAlias(todoUpdateCmd, "edit") {
		t.Fatalf("expected todo update to have edit alias, got %v", todoUpdateCmd.Aliases)
	}
}

func TestTodoDeleteHasDestroyAlias(t *testing.T) {
	if !containsAlias(todoDeleteCmd, "destroy") {
		t.Fatalf("expected todo delete to have destroy alias, got %v", todoDeleteCmd.Aliases)
	}
}

func containsAlias(cmd *cobra.Command, alias string) bool {
	for _, item := range cmd.Aliases {
		if item == alias {
			return true
		}
	}
	return false
}
