package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize incrementum configuration",
	Args:  cobra.NoArgs,
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	repoRoot, err := getRepoPath()
	if err != nil {
		return err
	}

	configPath := filepath.Join(repoRoot, "incrementum.toml")
	legacyConfigPath := filepath.Join(repoRoot, ".incrementum", "config.toml")

	exists, err := pathExists(configPath)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("configuration already exists: %s", configPath)
	}

	exists, err = pathExists(legacyConfigPath)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("configuration already exists: %s", legacyConfigPath)
	}

	testCommand, err := detectTestCommand(repoRoot)
	if err != nil {
		return err
	}

	configContents := renderInitConfig(testCommand)
	if err := os.WriteFile(configPath, []byte(configContents), 0644); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Created %s\n", configPath)
	if testCommand != "" {
		fmt.Fprintf(out, "Detected test command: %s\n", testCommand)
	} else {
		fmt.Fprintln(out, "No test command detected; please set job.test-commands.")
	}

	return nil
}

func detectTestCommand(repoRoot string) (string, error) {
	tasksPath := filepath.Join(repoRoot, "tasks.toml")
	goModPath := filepath.Join(repoRoot, "go.mod")
	packageJSONPath := filepath.Join(repoRoot, "package.json")

	tasksExists, err := pathExists(tasksPath)
	if err != nil {
		return "", err
	}
	goModExists, err := pathExists(goModPath)
	if err != nil {
		return "", err
	}
	if tasksExists {
		if goModExists {
			return "go tool run test", nil
		}
		return "run test", nil
	}

	binTestExists, err := binTestExists(repoRoot)
	if err != nil {
		return "", err
	}
	if binTestExists {
		return "./bin/test", nil
	}

	if goModExists {
		return "go test ./...", nil
	}

	packageJSONExists, err := pathExists(packageJSONPath)
	if err != nil {
		return "", err
	}
	if packageJSONExists {
		return "npm test", nil
	}

	return "", nil
}

func binTestExists(repoRoot string) (bool, error) {
	paths := []string{
		filepath.Join(repoRoot, "bin", "test"),
		filepath.Join(repoRoot, "bin", "test.sh"),
		filepath.Join(repoRoot, "bin", "test.bash"),
		filepath.Join(repoRoot, "bin", "test.fish"),
		filepath.Join(repoRoot, "bin", "test.zsh"),
	}

	for _, path := range paths {
		exists, err := pathExists(path)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}

	return false, nil
}

func renderInitConfig(testCommand string) string {
	if testCommand != "" {
		return fmt.Sprintf("[job]\ntest-commands = [%q]\n", testCommand)
	}

	return "[job]\n# test-commands = [\"go test ./...\"]\n# Set job.test-commands to run your tests.\n"
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
