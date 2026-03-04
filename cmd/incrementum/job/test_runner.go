package job

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// RunTestCommands executes test commands sequentially in a directory.
func RunTestCommands(dir string, commands []string) ([]TestCommandResult, error) {
	results := make([]TestCommandResult, 0, len(commands))
	for _, command := range commands {
		command = internalstrings.TrimSpace(command)
		if command == "" {
			return results, fmt.Errorf("test command is required")
		}

		cmd := exec.Command("/bin/bash", "-lc", command)
		cmd.Dir = dir
		var output bytes.Buffer
		writer := io.MultiWriter(os.Stdout, &output)
		cmd.Stdout = writer
		cmd.Stderr = writer
		cmd.Stdin = os.Stdin

		exitCode := 0
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				return results, fmt.Errorf("run test command %q: %w", command, err)
			}
			exitCode = exitErr.ExitCode()
		}

		results = append(results, TestCommandResult{
			Command:  command,
			ExitCode: exitCode,
			Output:   output.String(),
		})
	}

	return results, nil
}
