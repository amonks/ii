package main

import (
	"fmt"
	"strings"
	"time"

	"monks.co/ii/internal/db"
	"monks.co/ii/internal/paths"
	"monks.co/ii/workflow"

	"github.com/spf13/cobra"
)

var listAllFlag bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List workflow executions",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func init() {
	listCmd.Flags().BoolVar(&listAllFlag, "all", false, "Show all executions (not just active)")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
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

	records, err := workflow.ListExecutions(store.SqlDB(), repo, listAllFlag)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		if !listAllFlag {
			fmt.Fprintln(cmd.OutOrStdout(), "No active executions. Use --all to see all.")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "No executions found.")
		}
		return nil
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%-20s  %-10s  %-10s  %-20s  %s\n", "ID", "WORKFLOW", "STATUS", "NODE", "AGE")
	for _, r := range records {
		age := formatAge(r.CreatedAt)
		id := r.ID
		if len(id) > 20 {
			id = id[:20]
		}
		fmt.Fprintf(out, "%-20s  %-10s  %-10s  %-20s  %s\n",
			id, r.WorkflowName, r.Status, r.CurrentNode, age)
	}
	return nil
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// formatDuration formats a duration between two times.
func formatDuration(start, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return ""
	}
	d := end.Sub(start)
	return strings.TrimRight(strings.TrimRight(d.Truncate(time.Second).String(), "0"), ".")
}
