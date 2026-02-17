package main

import "slices"

import "github.com/spf13/cobra"

func hasChangedFlags(cmd *cobra.Command, flags ...string) bool {
	return slices.ContainsFunc(flags, cmd.Flags().Changed)
}
