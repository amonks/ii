package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"monks.co/ii/internal/db"
	"monks.co/ii/internal/paths"
	"monks.co/ii/workflow"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <workflow> [args]",
	Short: "Run a workflow",
	Long:  "Run a named workflow. Use 'ii run --list' to see available workflows.",
}

var runListFlag bool

func init() {
	runCmd.PersistentFlags().BoolVar(&runListFlag, "list", false, "List available workflows")
	rootCmd.AddCommand(runCmd)

	// Register each workflow as a subcommand.
	for name, wf := range workflow.Registry {
		cmd := buildRunSubcommand(name, wf)
		runCmd.AddCommand(cmd)
	}
}

func buildRunSubcommand(name string, wf workflow.Workflow) *cobra.Command {
	// Build usage string from inputs.
	var usage strings.Builder
	usage.WriteString(name)
	for _, input := range wf.Inputs {
		if input.Required {
			usage.WriteString(" <" + input.Name + ">")
		}
	}

	cmd := &cobra.Command{
		Use:   usage.String(),
		Short: fmt.Sprintf("Run the %s workflow", name),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflow(cmd, wf, args)
		},
	}

	// Register optional inputs as flags.
	for _, input := range wf.Inputs {
		if !input.Required {
			cmd.Flags().String(input.Name, input.Default, "")
		}
	}

	return cmd
}

func runWorkflow(cmd *cobra.Command, wf workflow.Workflow, args []string) error {
	// Map positional args to required inputs.
	inputs := make(map[string]string)
	argIdx := 0
	for _, input := range wf.Inputs {
		if input.Required {
			if argIdx >= len(args) {
				return fmt.Errorf("missing required argument: %s", input.Name)
			}
			inputs[input.Name] = args[argIdx]
			argIdx++
		}
	}

	// Map flags to optional inputs.
	for _, input := range wf.Inputs {
		if !input.Required {
			if val, err := cmd.Flags().GetString(input.Name); err == nil && val != "" {
				inputs[input.Name] = val
			}
		}
	}

	// Resolve workspace (cwd).
	workspace, err := paths.WorkingDir()
	if err != nil {
		return err
	}

	// Resolve scratchpad base.
	stateDir, err := paths.ResolveWithDefault(
		os.Getenv("INCREMENTUM_STATE_DIR"),
		paths.DefaultStateDir,
	)
	if err != nil {
		return fmt.Errorf("resolve state dir: %w", err)
	}

	// Open database.
	dbPath, err := paths.DefaultDBPath()
	if err != nil {
		return err
	}
	store, err := db.Open(dbPath, db.OpenOptions{})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	// Resolve repo name.
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}
	repo, err := db.GetOrCreateRepoName(store.SqlDB(), repoPath)
	if err != nil {
		return fmt.Errorf("resolve repo: %w", err)
	}

	// Set up context with signal handling.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	exec, err := workflow.Run(ctx, wf, workflow.RunOptions{
		DB:             store.SqlDB(),
		Workspace:      workspace,
		ScratchpadBase: stateDir + "/scratchpads",
		Inputs:         inputs,
		Repo:           repo,
	})
	if err != nil {
		if exec != nil {
			fmt.Fprintf(os.Stderr, "workflow %s %s at node %s\n", wf.Name, exec.Status, exec.CurrentNode)
		}
		return err
	}

	fmt.Fprintf(os.Stderr, "workflow %s %s\n", wf.Name, exec.Status)
	return nil
}
