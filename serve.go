package main

import (
	"context"

	"monks.co/ii/internal/config"
	internalstrings "monks.co/ii/internal/strings"
	"monks.co/ii/serve"
	"github.com/spf13/cobra"
)

var serveRun = serve.Run

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run pooled workers and a merge loop",
	Args:  cobra.NoArgs,
	RunE:  runServe,
}

var serveWorkers int
var serveTarget string

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVar(&serveWorkers, "workers", 0, "Number of workers")
	serveCmd.Flags().StringVar(&serveTarget, "onto", "", "Target bookmark to merge onto")
}

func runServe(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	cfg, err := config.Load(repoPath)
	if err != nil {
		return err
	}

	workers := serveWorkers
	if !cmd.Flags().Changed("workers") {
		workers = cfg.Pool.Workers
	}

	target := serveTarget
	if !cmd.Flags().Changed("onto") {
		target = cfg.Merge.Target
	}
	if internalstrings.IsBlank(target) {
		target = "main"
	}

	runner, err := makeAgentRunnerFunc(repoPath)
	if err != nil {
		return err
	}
	defer runner.Close()

	runLLM, err := makeRunLLMFunc(repoPath, runner)
	if err != nil {
		return err
	}

	opts := serve.Options{
		RepoPath:    repoPath,
		Workers:     workers,
		Target:      target,
		RunLLM:      runLLM,
	}

	ctx := context.Background()
	return serveRun(ctx, opts)
}
