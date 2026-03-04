package listflags

import "github.com/spf13/cobra"

// AddAllFlag adds a shared --all flag to list commands.
func AddAllFlag(cmd *cobra.Command, target *bool) {
	if target == nil {
		cmd.Flags().Bool("all", false, "Include all statuses")
		return
	}

	cmd.Flags().BoolVar(target, "all", false, "Include all statuses")
}
