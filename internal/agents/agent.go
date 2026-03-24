package agents

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
)

// AgentAdapter invokes the extracted cmd/agent binary.
type AgentAdapter struct {
	Binary string // Path to agent binary (default: "agent")
}

var tokenUsageRe = regexp.MustCompile(`tokens: input=(\d+) output=(\d+) total=(\d+)`)

func (a *AgentAdapter) Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	binary := a.Binary
	if binary == "" {
		binary = "agent"
	}

	// Write prompt to temp file
	tmpFile, err := os.CreateTemp("", "agent-prompt-*.md")
	if err != nil {
		return RunResult{}, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(opts.Prompt); err != nil {
		tmpFile.Close()
		return RunResult{}, fmt.Errorf("write prompt file: %w", err)
	}
	tmpFile.Close()

	args := []string{"run", "--prompt-file", tmpFile.Name()}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.WorkDir != "" {
		args = append(args, "--workdir", opts.WorkDir)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = opts.WorkDir
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	result := RunResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	// Parse token usage from stderr
	if matches := tokenUsageRe.FindStringSubmatch(stderr.String()); len(matches) == 4 {
		result.InputTokens, _ = strconv.Atoi(matches[1])
		result.OutputTokens, _ = strconv.Atoi(matches[2])
	}

	if err != nil {
		return result, fmt.Errorf("agent run: %w", err)
	}
	return result, nil
}
