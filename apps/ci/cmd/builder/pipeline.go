package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Pipeline orchestrates the full CI pipeline.
type Pipeline struct {
	Config   *Config
	Reporter *Reporter
}

// Run executes the pipeline steps in order.
func (p *Pipeline) Run(ctx context.Context) error {
	// Step 1: Fetch latest code (updates p.Config.Root to persistent volume).
	slog.Info("fetching latest code")
	if err := p.fetchCode(); err != nil {
		return fmt.Errorf("fetching code: %w", err)
	}

	root := p.Config.Root

	// Step 2: Run generate + test.
	slog.Info("running tests")
	if err := RunTests(ctx, root, p.Reporter); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	// Steps 3-5: Deploy, publish, and terraform run concurrently.
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	wg.Add(3)

	go func() {
		defer wg.Done()
		slog.Info("deploying affected apps")
		if err := DeployAffected(root, p.Config.HeadSHA, p.Config.BaseSHA, p.Config.FlyAPIToken, p.Config.BaseImageRef, p.Reporter); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("deploy failed: %w", err))
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		slog.Info("publishing subtrees")
		if err := PublishSubtrees(root, p.Reporter); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("publish failed: %w", err))
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		slog.Info("applying terraform")
		if err := TerraformApply(root, p.Reporter); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("terraform failed: %w", err))
			mu.Unlock()
		}
	}()

	wg.Wait()

	return errors.Join(errs...)
}

func (p *Pipeline) fetchCode() error {
	start := time.Now()
	p.Reporter.StartJob("fetch", "fetch")

	w := p.Reporter.StreamWriter("fetch", "output")
	defer w.Close()

	repoDir := filepath.Join(p.Config.DataDir, "repo")

	var err error
	if _, statErr := os.Stat(filepath.Join(repoDir, ".jj")); os.IsNotExist(statErr) {
		fmt.Fprintf(w, "=== cloning repo to %s\n", repoDir)
		err = p.runCmd(w, "", "jj", "git", "clone", p.Config.RepoURL, repoDir, "--colocate")
	} else {
		fmt.Fprintf(w, "=== fetching latest code in %s\n", repoDir)
		err = p.runCmd(w, repoDir, "jj", "git", "fetch")
	}

	if err == nil {
		fmt.Fprintf(w, "=== checking out %s\n", p.Config.HeadSHA)
		err = p.runCmd(w, repoDir, "jj", "new", p.Config.HeadSHA)
	}

	if err == nil {
		p.Config.Root = repoDir
	}

	duration := time.Since(start).Milliseconds()
	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
		fmt.Fprintf(w, "=== fetch failed: %s\n", errMsg)
	} else {
		fmt.Fprintf(w, "=== fetched in %dms\n", duration)
	}

	p.Reporter.FinishJob("fetch", FinishJobResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})

	return err
}

func (p *Pipeline) runCmd(w *StreamWriter, dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}
