package main

import (
	"context"
	"fmt"

	"monks.co/incrementum/internal/config"
	internalstrings "monks.co/incrementum/internal/strings"
	"monks.co/incrementum/merge"
	"github.com/spf13/cobra"
)

var mergeRun = merge.Merge

var mergeCmd = &cobra.Command{
	Use:   "merge <change-id>",
	Short: "Merge a change onto a target bookmark",
	Args:  cobra.ExactArgs(1),
	RunE:  runMerge,
}

var mergeTarget string

func init() {
	rootCmd.AddCommand(mergeCmd)
	mergeCmd.Flags().StringVar(&mergeTarget, "onto", "", "Target bookmark to merge onto")
}

func runMerge(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	workspacePath, err := resolveWorkspaceRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Load(repoPath)
	if err != nil {
		return err
	}
	target := mergeTarget
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

	opts := merge.Options{
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		ChangeID:      args[0],
		Target:        target,
		RunLLM:        runLLM,
	}
	if err := mergeRun(context.Background(), opts); err != nil {
		return err
	}

	fmt.Printf("Merged %s onto %s\n", args[0], target)
	return nil
}
