package agents

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// ClaudeAdapter invokes Claude Code (the claude CLI).
type ClaudeAdapter struct {
	Binary string // Path to claude binary (default: "claude")
}

func (a *ClaudeAdapter) Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	binary := a.Binary
	if binary == "" {
		binary = "claude"
	}

	args := []string{
		"-p", opts.Prompt,
		"--max-turns", "50",
		"--output-format", "text",
		"--permission-mode", "bypassPermissions",
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

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

	// Claude Code doesn't expose token usage in text output mode
	// InputTokens and OutputTokens remain 0

	if err != nil {
		return result, fmt.Errorf("claude run: %w", err)
	}
	return result, nil
}
