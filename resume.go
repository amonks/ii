package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"monks.co/ii/internal/db"
	"monks.co/ii/internal/paths"
	"monks.co/ii/workflow"

	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume <execution-id>",
	Short: "Resume an interrupted or failed workflow execution",
	Args:  cobra.ExactArgs(1),
	RunE:  runResume,
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}

func runResume(cmd *cobra.Command, args []string) error {
	stateDir, err := paths.ResolveWithDefault(
		os.Getenv("INCREMENTUM_STATE_DIR"),
		paths.DefaultStateDir,
	)
	if err != nil {
		return fmt.Errorf("resolve state dir: %w", err)
	}

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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	exec, err := workflow.Resume(ctx, args[0], workflow.ResumeOptions{
		DB:             store.SqlDB(),
		ScratchpadBase: stateDir + "/scratchpads",
		Repo:           repo,
	})
	if err != nil {
		if exec != nil {
			fmt.Fprintf(os.Stderr, "workflow %s at node %s\n", exec.Status, exec.CurrentNode)
		}
		return err
	}

	fmt.Fprintf(os.Stderr, "workflow %s %s\n", exec.WorkflowName, exec.Status)
	return nil
}
