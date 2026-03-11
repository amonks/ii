package agents

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// CodexAdapter invokes the Codex CLI.
type CodexAdapter struct {
	Binary string // Path to codex binary (default: "codex")
}

func (a *CodexAdapter) Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	binary := a.Binary
	if binary == "" {
		binary = "codex"
	}

	args := []string{"exec", opts.Prompt}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = opts.WorkDir
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := RunResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	// Codex doesn't expose token usage
	// InputTokens and OutputTokens remain 0

	if err != nil {
		return result, fmt.Errorf("codex run: %w", err)
	}
	return result, nil
}
