package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"monks.co/incrementum/internal/testsupport"
	"monks.co/incrementum/merge"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/spf13/cobra"
)

func TestMergeScripts(t *testing.T) {
	oldRun := mergeRun
	mergeRun = func(ctx context.Context, opts merge.Options) error {
		return nil
	}
	t.Cleanup(func() { mergeRun = oldRun })

	testscript.Run(t, testscript.Params{
		Dir: "testdata/merge",
		Setup: func(env *testscript.Env) error {
			if err := testsupport.SetupScriptEnv(t, env); err != nil {
				return err
			}
			repoDir := filepath.Join(env.WorkDir, "repo")
			return os.MkdirAll(repoDir, 0o755)
		},
	})
}

func TestMergeCommandUsesConfigDefaults(t *testing.T) {
	repoDir := t.TempDir()
	if err := runMergeCommand(repoDir, "jj", "git", "init"); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	configPath := filepath.Join(repoDir, "incrementum.toml")
	configContent := "[merge]\ntarget = \"mainline\"\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	errSentinel := errors.New("merge run")
	oldRun := mergeRun
	mergeRun = func(ctx context.Context, opts merge.Options) error {
		if opts.Target != "mainline" {
			return errors.New("unexpected target")
		}
		if opts.ChangeID != "abc123" {
			return errors.New("unexpected change id")
		}
		return errSentinel
	}
	t.Cleanup(func() { mergeRun = oldRun })

	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&mergeTarget, "onto", "", "")
	if err := runMerge(cmd, []string{"abc123"}); err != nil {
		if !errors.Is(err, errSentinel) {
			if !strings.Contains(err.Error(), "unexpected") {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		return
	}
	t.Fatalf("expected error from mergeRun")
}

func TestMergeCommandOntoFlagOverridesConfig(t *testing.T) {
	repoDir := t.TempDir()
	if err := runMergeCommand(repoDir, "jj", "git", "init"); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	configPath := filepath.Join(repoDir, "incrementum.toml")
	configContent := "[merge]\ntarget = \"mainline\"\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	errSentinel := errors.New("merge run")
	oldRun := mergeRun
	mergeRun = func(ctx context.Context, opts merge.Options) error {
		if opts.Target != "dev" {
			return errors.New("unexpected target")
		}
		return errSentinel
	}
	t.Cleanup(func() { mergeRun = oldRun })

	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	mergeTarget = "dev"
	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&mergeTarget, "onto", "", "")
	_ = cmd.Flags().Set("onto", "dev")
	if err := runMerge(cmd, []string{"abc123"}); err != nil {
		if !errors.Is(err, errSentinel) {
			if !strings.Contains(err.Error(), "unexpected") {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		return
	}
	t.Fatalf("expected error from mergeRun")
}

func TestMergeCommandFlagValues(t *testing.T) {
	oldRun := mergeRun
	mergeRun = func(ctx context.Context, opts merge.Options) error {
		return nil
	}
	t.Cleanup(func() { mergeRun = oldRun })

	repoDir := t.TempDir()
	if err := runMergeCommand(repoDir, "jj", "git", "init"); err != nil {
		t.Fatalf("init repo: %v", err)
	}
	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	mergeTarget = "main"
	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&mergeTarget, "onto", "", "")
	_ = cmd.Flags().Set("onto", "main")
	if err := runMerge(cmd, []string{"abc123"}); err != nil {
		t.Fatalf("run merge: %v", err)
	}
}

func runMergeCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	return nil
}
