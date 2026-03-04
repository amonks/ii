package main

import (
	"fmt"
	"strings"

	"github.com/amonks/incrementum/job"
	"github.com/spf13/cobra"
)

var helpCmd = &cobra.Command{
	Use:   "help [command]",
	Short: "Help about any command",
	Args:  cobra.ArbitraryArgs,
	RunE:  runHelp,
}

var helpTemplatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Show prompt template variables",
	Args:  cobra.NoArgs,
	RunE:  runHelpTemplates,
}

func init() {
	rootCmd.SetHelpCommand(helpCmd)
	helpCmd.AddCommand(helpTemplatesCmd)
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

func runHelpTemplates(cmd *cobra.Command, args []string) error {
	info, err := job.DefaultPromptTemplateInfo()
	if err != nil {
		return err
	}
	var builder strings.Builder
	for i, template := range info {
		if i > 0 {
			builder.WriteString("\n")
		}
		fmt.Fprintf(&builder, "%s\n", template.Name)
		fmt.Fprintf(&builder, "  Override: %s\n", job.PromptOverridePath(template.Name))
		builder.WriteString("  Variables:\n")
		for _, variable := range template.Variables {
			fmt.Fprintf(&builder, "    - %s (%s)\n", variable.Name, variable.Type)
		}
		builder.WriteString("  Template:\n")
		builder.WriteString("```\n")
		builder.WriteString(template.Contents)
		if !strings.HasSuffix(template.Contents, "\n") {
			builder.WriteString("\n")
		}
		builder.WriteString("```\n")
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), builder.String())
	return err
}
