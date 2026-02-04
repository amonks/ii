package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionString(t *testing.T) {
	prevChangeID := buildChangeID
	prevCommitID := buildCommitID
	t.Cleanup(func() {
		buildChangeID = prevChangeID
		buildCommitID = prevCommitID
	})

	buildChangeID = "change123"
	buildCommitID = "commit456"

	got := versionString()
	want := "change_id change123\ncommit_id commit456"
	if got != want {
		t.Fatalf("expected version string %q, got %q", want, got)
	}
}

func TestRootCommandHasVersion(t *testing.T) {
	if rootCmd.Version == "" {
		t.Fatal("expected root command version to be set")
	}
}

// TestVersionLdflagsInjection verifies that -ldflags -X main.<var> correctly
// injects build metadata into the binary. This catches regressions where the
// linker symbol path is incorrect (e.g. using package path instead of main.).
func TestVersionLdflagsInjection(t *testing.T) {
	// Find module root
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	moduleRoot := cwd
	for {
		if _, err := os.Stat(filepath.Join(moduleRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(moduleRoot)
		if parent == moduleRoot {
			t.Fatal("could not find module root (go.mod)")
		}
		moduleRoot = parent
	}

	// Build binary with test ldflags
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "ii")
	testChangeID := "test-change-abc123"
	testCommitID := "test-commit-def456"
	ldflags := "-X main.buildChangeID=" + testChangeID + " -X main.buildCommitID=" + testCommitID

	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", binPath, "./cmd/ii")
	cmd.Dir = moduleRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build binary: %v: %s", err, output)
	}

	// Run the binary with -version flag
	cmd = exec.Command(binPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run binary: %v: %s", err, output)
	}

	// Verify the injected values appear in output
	got := string(output)
	if !strings.Contains(got, testChangeID) {
		t.Errorf("version output missing change_id %q:\n%s", testChangeID, got)
	}
	if !strings.Contains(got, testCommitID) {
		t.Errorf("version output missing commit_id %q:\n%s", testCommitID, got)
	}
}
