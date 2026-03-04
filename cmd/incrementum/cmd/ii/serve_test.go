package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amonks/incrementum/internal/testsupport"
	"github.com/amonks/incrementum/serve"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/spf13/cobra"
)

func TestServeScripts(t *testing.T) {
	oldRun := serveRun
	serveRun = func(ctx context.Context, opts serve.Options) error {
		return nil
	}
	t.Cleanup(func() { serveRun = oldRun })

	testscript.Run(t, testscript.Params{
		Dir: "testdata/serve",
		Setup: func(env *testscript.Env) error {
			if err := testsupport.SetupScriptEnv(t, env); err != nil {
				return err
			}
			repoDir := filepath.Join(env.WorkDir, "repo")
			return os.MkdirAll(repoDir, 0o755)
		},
	})
}

func TestServeCommandUsesConfigDefaults(t *testing.T) {
	repoDir := t.TempDir()
	if err := runServeCommand(repoDir, "jj", "git", "init"); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	configPath := filepath.Join(repoDir, "incrementum.toml")
	configContent := "[pool]\nworkers = 3\n\n[merge]\ntarget = \"mainline\"\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	errSentinel := errors.New("serve run")
	oldRun := serveRun
	serveRun = func(ctx context.Context, opts serve.Options) error {
		if opts.Workers != 3 {
			return errors.New("unexpected workers")
		}
		if opts.Target != "mainline" {
			return errors.New("unexpected target")
		}
		return errSentinel
	}
	t.Cleanup(func() { serveRun = oldRun })

	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := runServe(&cobra.Command{}, nil); err != nil {
		if !errors.Is(err, errSentinel) {
			if !strings.Contains(err.Error(), "unexpected") {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		return
	}
	t.Fatalf("expected error from serveRun")
}

func TestServeCommandFlagsOverrideConfig(t *testing.T) {
	repoDir := t.TempDir()
	if err := runServeCommand(repoDir, "jj", "git", "init"); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	configPath := filepath.Join(repoDir, "incrementum.toml")
	configContent := "[pool]\nworkers = 3\n\n[merge]\ntarget = \"mainline\"\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	errSentinel := errors.New("serve run")
	oldRun := serveRun
	serveRun = func(ctx context.Context, opts serve.Options) error {
		if opts.Workers != 2 {
			return errors.New("unexpected workers")
		}
		if opts.Target != "dev" {
			return errors.New("unexpected target")
		}
		return errSentinel
	}
	t.Cleanup(func() { serveRun = oldRun })

	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	serveWorkers = 2
	serveTarget = "dev"
	cmd := &cobra.Command{}
	cmd.Flags().IntVar(&serveWorkers, "workers", 0, "")
	cmd.Flags().StringVar(&serveTarget, "onto", "", "")
	_ = cmd.Flags().Set("workers", "2")
	_ = cmd.Flags().Set("onto", "dev")
	if err := runServe(cmd, nil); err != nil {
		if !errors.Is(err, errSentinel) {
			if !strings.Contains(err.Error(), "unexpected") {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		return
	}
	t.Fatalf("expected error from serveRun")
}

func TestServeCommandFlagValues(t *testing.T) {
	oldRun := serveRun
	serveRun = func(ctx context.Context, opts serve.Options) error {
		return nil
	}
	t.Cleanup(func() { serveRun = oldRun })

	repoDir := t.TempDir()
	if err := runServeCommand(repoDir, "jj", "git", "init"); err != nil {
		t.Fatalf("init repo: %v", err)
	}
	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	serveWorkers = 1
	serveTarget = "main"
	cmd := &cobra.Command{}
	cmd.Flags().IntVar(&serveWorkers, "workers", 0, "")
	cmd.Flags().StringVar(&serveTarget, "onto", "", "")
	_ = cmd.Flags().Set("workers", "1")
	_ = cmd.Flags().Set("onto", "main")
	if err := runServe(cmd, nil); err != nil {
		t.Fatalf("run serve: %v", err)
	}
}

func runServeCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	return nil
}
