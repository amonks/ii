package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentsPrelude_WhenMissing_ReturnsEmpty(t *testing.T) {
	prelude, err := agentsPrelude(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prelude != "" {
		t.Fatalf("expected empty prelude, got %q", prelude)
	}
}

func TestAgentsPrelude_WhenPresent_ReturnsTrimmedWithBlankLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("  hello\nworld\n\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prelude, err := agentsPrelude(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "hello\nworld\n\n"
	if prelude != want {
		t.Fatalf("unexpected prelude.\nwant: %q\n got: %q", want, prelude)
	}
}
