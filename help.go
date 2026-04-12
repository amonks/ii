package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var helpCmd = &cobra.Command{
	Use:   "help [command]",
	Short: "Help about any command",
	Args:  cobra.ArbitraryArgs,
	RunE:  runHelp,
}

func init() {
	rootCmd.SetHelpCommand(helpCmd)
}

func runHelp(cmd *cobra.Command, args []string) error {
	root := cmd.Root()
	if len(args) == 0 {
		return root.Help()
	}

	target, _, err := root.Find(args)
	if err != nil || target == nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Unknown help topic %q\n", strings.Join(args, " "))
		return root.Help()
	}

	return target.Help()
}
