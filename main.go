// Package main implements the ii CLI tool.
package main

import (
	"errors"
	"os"

	"monks.co/ii/internal/paths"

	"github.com/spf13/cobra"
)

func main() {
	os.Args = normalizeVersionArgs(os.Args)
	if err := rootCmd.Execute(); err != nil {
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

func normalizeVersionArgs(args []string) []string {
	if len(args) < 2 {
		return args
	}

	normalized := make([]string, 0, len(args))
	normalized = append(normalized, args[0])
	for _, arg := range args[1:] {
		if arg == "-version" {
			arg = "--version"
		}
		normalized = append(normalized, arg)
	}
	return normalized
}

var rootCmd = &cobra.Command{
	Use:   "ii",
	Short: "Incrementum - tools for incremental development",
}

// getRepoPath returns the jj repository root for the current directory.
func getRepoPath() (string, error) {
	cwd, err := paths.WorkingDir()
	if err != nil {
		return "", err
	}

	return resolveRepoRoot(cwd)
}
