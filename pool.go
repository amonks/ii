package main

import (
	"context"

	"monks.co/ii/internal/config"
	"monks.co/ii/pool"
	"github.com/spf13/cobra"
)

var poolRun = pool.Run

var poolCmd = &cobra.Command{
	Use:   "pool",
	Short: "Run job workers in pooled workspaces",
	Args:  cobra.NoArgs,
	RunE:  runPool,
}

var poolWorkers int

func init() {
	rootCmd.AddCommand(poolCmd)
	poolCmd.Flags().IntVar(&poolWorkers, "workers", 0, "Number of workers")
}

func runPool(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	cfg, err := config.Load(repoPath)
	if err != nil {
		return err
	}
	workers := poolWorkers
	if !cmd.Flags().Changed("workers") {
		workers = cfg.Pool.Workers
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

	opts := pool.Options{
		RepoPath:    repoPath,
		Workers:     workers,
		RunLLM:      runLLM,
	}

	ctx := context.Background()
	return poolRun(ctx, opts)
}
