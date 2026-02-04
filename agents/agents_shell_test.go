package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeExecutable(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestClaudeRunnerExecutesCLI(t *testing.T) {
	tmpDir := t.TempDir()
	writeExecutable(t, tmpDir, "claude", "#!/bin/sh\nif [ \"$1\" != \"-p\" ] || [ \"$2\" != \"--dangerously-skip-permissions\" ]; then\n  echo \"expected -p --dangerously-skip-permissions\" >&2\n  exit 2\nfi\ncat >/dev/null\nexit 0\n")
	t.Setenv("PATH", tmpDir)

	runner := NewClaudeRunner()
	handle, err := runner.Run(context.Background(), RunOptions{Prompt: "hello"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.HasPrefix(result.SessionID, "external-claude-") {
		t.Fatalf("expected claude session id, got %q", result.SessionID)
	}
}

func TestCodexRunnerExecutesCLI(t *testing.T) {
	tmpDir := t.TempDir()
	writeExecutable(t, tmpDir, "codex", "#!/bin/sh\nif [ \"$1\" != \"exec\" ] || [ \"$2\" != \"--skip-git-repo-check\" ]; then\n  echo \"expected exec --skip-git-repo-check\" >&2\n  exit 2\nfi\ncat >/dev/null\nexit 0\n")
	t.Setenv("PATH", tmpDir)

	runner := NewCodexRunner()
	handle, err := runner.Run(context.Background(), RunOptions{Prompt: "hello"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.HasPrefix(result.SessionID, "external-codex-") {
		t.Fatalf("expected codex session id, got %q", result.SessionID)
	}
}
