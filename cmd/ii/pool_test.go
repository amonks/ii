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
	"github.com/amonks/incrementum/pool"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/spf13/cobra"
)

func TestPoolScripts(t *testing.T) {
	oldRun := poolRun
	poolRun = func(ctx context.Context, opts pool.Options) error {
		return nil
	}
	t.Cleanup(func() { poolRun = oldRun })

	testscript.Run(t, testscript.Params{
		Dir: "testdata/pool",
		Setup: func(env *testscript.Env) error {
			if err := testsupport.SetupScriptEnv(t, env); err != nil {
				return err
			}
			repoDir := filepath.Join(env.WorkDir, "repo")
			return os.MkdirAll(repoDir, 0o755)
		},
	})
}

func TestPoolCommandUsesConfigDefaults(t *testing.T) {
	repoDir := t.TempDir()
	if err := runCommand(repoDir, "jj", "git", "init"); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	configPath := filepath.Join(repoDir, "incrementum.toml")
	configContent := "[pool]\nworkers = 3\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	errSentinel := errors.New("pool run")
	oldRun := poolRun
	poolRun = func(ctx context.Context, opts pool.Options) error {
		if opts.Workers != 3 {
			return errors.New("unexpected workers")
		}
		return errSentinel
	}
	t.Cleanup(func() { poolRun = oldRun })

	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := runPool(&cobra.Command{}, nil); err != nil {
		if !errors.Is(err, errSentinel) {
			if !strings.Contains(err.Error(), "unexpected workers") {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		return
	}
	t.Fatalf("expected error from poolRun")
}

func TestPoolCommandWorkersFlagOverridesConfig(t *testing.T) {
	repoDir := t.TempDir()
	if err := runCommand(repoDir, "jj", "git", "init"); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	configPath := filepath.Join(repoDir, "incrementum.toml")
	configContent := "[pool]\nworkers = 3\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	errSentinel := errors.New("pool run")
	oldRun := poolRun
	poolRun = func(ctx context.Context, opts pool.Options) error {
		if opts.Workers != 2 {
			return errors.New("unexpected workers")
		}
		return errSentinel
	}
	t.Cleanup(func() { poolRun = oldRun })

	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	poolWorkers = 2
	cmd := &cobra.Command{}
	cmd.Flags().IntVar(&poolWorkers, "workers", 0, "")
	_ = cmd.Flags().Set("workers", "2")
	if err := runPool(cmd, nil); err != nil {
		if !errors.Is(err, errSentinel) {
			if !strings.Contains(err.Error(), "unexpected workers") {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		return
	}
	t.Fatalf("expected error from poolRun")
}

func TestPoolCommandWorkerFlagValue(t *testing.T) {
	oldRun := poolRun
	poolRun = func(ctx context.Context, opts pool.Options) error {
		return nil
	}
	t.Cleanup(func() { poolRun = oldRun })

	repoDir := t.TempDir()
	if err := runCommand(repoDir, "jj", "git", "init"); err != nil {
		t.Fatalf("init repo: %v", err)
	}
	cwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	poolWorkers = 1
	cmd := &cobra.Command{}
	cmd.Flags().IntVar(&poolWorkers, "workers", 0, "")
	_ = cmd.Flags().Set("workers", "1")
	if err := runPool(cmd, nil); err != nil {
		t.Fatalf("run pool: %v", err)
	}
}

func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	return nil
}
