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
	prevDev := buildDev
	t.Cleanup(func() {
		buildChangeID = prevChangeID
		buildCommitID = prevCommitID
		buildDev = prevDev
	})

	buildChangeID = "change123"
	buildCommitID = "commit456"
	buildDev = ""

	got := versionString()
	want := "change_id change123\ncommit_id commit456"
	if got != want {
		t.Fatalf("expected version string %q, got %q", want, got)
	}
}

func TestVersionStringDev(t *testing.T) {
	prevChangeID := buildChangeID
	prevCommitID := buildCommitID
	prevDev := buildDev
	t.Cleanup(func() {
		buildChangeID = prevChangeID
		buildCommitID = prevCommitID
		buildDev = prevDev
	})

	buildChangeID = "change123"
	buildCommitID = "commit456"
	buildDev = "true"

	got := versionString()
	want := "dev build\nchange_id change123\ncommit_id commit456"
	if got != want {
		t.Fatalf("expected version string %q, got %q", want, got)
	}
}

func TestBuildVersion(t *testing.T) {
	prevChangeID := buildChangeID
	prevCommitID := buildCommitID
	prevDev := buildDev
	t.Cleanup(func() {
		buildChangeID = prevChangeID
		buildCommitID = prevCommitID
		buildDev = prevDev
	})

	buildChangeID = "abc123"
	buildCommitID = "def456"

	buildDev = ""
	if got := buildVersion(); got != "abc123:def456" {
		t.Fatalf("buildVersion() = %q, want %q", got, "abc123:def456")
	}

	buildDev = "true"
	if got := buildVersion(); got != "dev" {
		t.Fatalf("buildVersion() dev = %q, want %q", got, "dev")
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

	testChangeID := "test-change-abc123"
	testCommitID := "test-commit-def456"

	t.Run("installed", func(t *testing.T) {
		binDir := t.TempDir()
		binPath := filepath.Join(binDir, "ii")
		ldflags := "-X main.buildChangeID=" + testChangeID + " -X main.buildCommitID=" + testCommitID

		cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", binPath, ".")
		cmd.Dir = moduleRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build binary: %v: %s", err, output)
		}

		cmd = exec.Command(binPath, "-version")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run binary: %v: %s", err, output)
		}

		got := string(output)
		if !strings.Contains(got, testChangeID) {
			t.Errorf("version output missing change_id %q:\n%s", testChangeID, got)
		}
		if !strings.Contains(got, testCommitID) {
			t.Errorf("version output missing commit_id %q:\n%s", testCommitID, got)
		}
		if strings.Contains(got, "dev build") {
			t.Errorf("installed build should not say dev build:\n%s", got)
		}
	})

	t.Run("dev", func(t *testing.T) {
		binDir := t.TempDir()
		binPath := filepath.Join(binDir, "ii")
		ldflags := "-X main.buildChangeID=" + testChangeID + " -X main.buildCommitID=" + testCommitID + " -X main.buildDev=true"

		cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", binPath, ".")
		cmd.Dir = moduleRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build binary: %v: %s", err, output)
		}

		cmd = exec.Command(binPath, "-version")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run binary: %v: %s", err, output)
		}

		got := string(output)
		if !strings.Contains(got, "dev build") {
			t.Errorf("dev build should say 'dev build':\n%s", got)
		}
		if !strings.Contains(got, testChangeID) {
			t.Errorf("version output missing change_id %q:\n%s", testChangeID, got)
		}
		if !strings.Contains(got, testCommitID) {
			t.Errorf("version output missing commit_id %q:\n%s", testCommitID, got)
		}
	})
}
