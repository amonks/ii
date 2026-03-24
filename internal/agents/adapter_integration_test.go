package agents_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"monks.co/ii/internal/agents"
)

// TestClaudeAdapter_E2E exercises the ClaudeAdapter with the real claude binary.
func TestClaudeAdapter_E2E(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude binary not found in PATH")
	}
	if os.Getuid() == 0 {
		t.Skip("claude refuses --permission-mode bypassPermissions as root")
	}

	// Clear env vars that prevent nested Claude Code invocations.
	for _, key := range []string{"CLAUDE_CODE_ENTRYPOINT", "CLAUDECODE"} {
		if prev, ok := os.LookupEnv(key); ok {
			os.Unsetenv(key)
			t.Cleanup(func() { os.Setenv(key, prev) })
		}
	}

	adapter := &agents.ClaudeAdapter{}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := adapter.Run(ctx, agents.RunOptions{
		Prompt: "What is 2+2? Reply with just the number.",
	})
	if err != nil {
		t.Fatalf("ClaudeAdapter.Run failed: %v\nstderr: %s", err, result.Stderr)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d; stderr: %s", result.ExitCode, result.Stderr)
	}

	if !strings.Contains(result.Stdout, "4") {
		t.Errorf("expected stdout to contain '4', got %q", result.Stdout)
	}

	t.Logf("claude stdout: %s", strings.TrimSpace(result.Stdout))
}

// TestCodexAdapter_E2E exercises the CodexAdapter with the real codex binary.
func TestCodexAdapter_E2E(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex binary not found in PATH")
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	adapter := &agents.CodexAdapter{}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := adapter.Run(ctx, agents.RunOptions{
		Prompt: "What is 2+2? Reply with just the number.",
	})
	if err != nil {
		t.Fatalf("CodexAdapter.Run failed: %v\nstderr: %s", err, result.Stderr)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d; stderr: %s", result.ExitCode, result.Stderr)
	}

	if !strings.Contains(result.Stdout, "4") {
		t.Errorf("expected stdout to contain '4', got %q", result.Stdout)
	}

	t.Logf("codex stdout: %s", strings.TrimSpace(result.Stdout))
}
