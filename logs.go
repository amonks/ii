package main

import (
	"fmt"
	"strings"

	"monks.co/ii/internal/db"
	"monks.co/ii/internal/paths"
	"monks.co/ii/workflow"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <execution-id>",
	Short: "Show the node trace for a workflow execution",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	dbPath, err := paths.DefaultDBPath()
	if err != nil {
		return err
	}
	store, err := db.Open(dbPath, db.OpenOptions{})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}
	repo, err := db.GetOrCreateRepoName(store.SqlDB(), repoPath)
	if err != nil {
		return fmt.Errorf("resolve repo: %w", err)
	}

	record, err := workflow.FindExecution(store.SqlDB(), repo, args[0])
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Execution: %s\n", record.ID)
	fmt.Fprintf(out, "Workflow:  %s\n", record.WorkflowName)
	fmt.Fprintf(out, "Status:    %s\n", record.Status)
	fmt.Fprintf(out, "Node:      %s\n", record.CurrentNode)
	fmt.Fprintf(out, "Workspace: %s\n", record.Workspace)
	fmt.Fprintf(out, "Created:   %s\n", record.CreatedAt.Format("2006-01-02 15:04:05"))
	if !record.CompletedAt.IsZero() {
		fmt.Fprintf(out, "Completed: %s\n", record.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Fprintln(out)

	nodeRuns, err := workflow.ListNodeRuns(store.SqlDB(), record.ID)
	if err != nil {
		return err
	}

	if len(nodeRuns) == 0 {
		fmt.Fprintln(out, "No node runs recorded.")
		return nil
	}

	for _, nr := range nodeRuns {
		exitStr := "?"
		if nr.ExitCode != nil {
			exitStr = fmt.Sprintf("%d", *nr.ExitCode)
		}
		duration := ""
		if !nr.CompletedAt.IsZero() {
			duration = fmt.Sprintf(" (%s)", formatDuration(nr.StartedAt, nr.CompletedAt))
		}
		fmt.Fprintf(out, "-> %s  exit=%s%s\n", nr.NodeName, exitStr, duration)

		// Show scratchpad diffs.
		diffs, err := workflow.ListDiffs(store.SqlDB(), nr.ID)
		if err != nil {
			return err
		}
		for _, d := range diffs {
			switch d.Op {
			case workflow.OpAdded:
				preview := previewContent(d.Content)
				fmt.Fprintf(out, "   + %s: %s\n", d.Path, preview)
			case workflow.OpModified:
				preview := previewContent(d.Content)
				fmt.Fprintf(out, "   ~ %s: %s\n", d.Path, preview)
			case workflow.OpDeleted:
				fmt.Fprintf(out, "   - %s\n", d.Path)
			}
		}
	}
	return nil
}

// previewContent returns a truncated single-line preview of file content.
func previewContent(content string) string {
	line := strings.SplitN(content, "\n", 2)[0]
	if len(line) > 60 {
		line = line[:60] + "..."
	}
	return line
}
