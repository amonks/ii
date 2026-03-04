// Package editor provides utilities for interactive editing with $EDITOR.
package editor

import (
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/term"
)

// IsInteractive returns true if stdin is a terminal.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// Edit opens the given file in $EDITOR (or vi as fallback) and waits for it to exit.
// Returns nil if the editor exits with status 0, otherwise returns an error.
func Edit(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("editor exited with status %d", exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run editor: %w", err)
	}

	return nil
}
