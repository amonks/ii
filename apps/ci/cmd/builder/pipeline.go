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

	// Step 3: Single deploy job. Analysis runs first, then all
	// streams (per-app deploys, image rebuilds, publish, terraform)
	// run as peer goroutines.
	p.Reporter.StartJob("deploy", "deploy")
	deployStart := time.Now()

	analysis, err := AnalyzeDeploy(root, p.Config.BaseSHA, p.Reporter)
	if err != nil {
		duration := time.Since(deployStart).Milliseconds()
		p.Reporter.FinishJob("deploy", FinishJobResult{
			Status:     "failed",
			DurationMs: duration,
			Error:      err.Error(),
		})
		return fmt.Errorf("deploy analysis: %w", err)
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	// Deploy affected apps and rebuild images.
	wg.Go(func() {
		if err := DeployAnalyzed(analysis, root, p.Config.HeadSHA, p.Config.FlyAPIToken, p.Config.BaseImageRef, p.Reporter); err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}
	})

	// Publish subtrees.
	wg.Go(func() {
		if err := PublishSubtrees(root, p.Reporter); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("publish failed: %w", err))
			mu.Unlock()
		}
	})

	// Terraform apply.
	wg.Go(func() {
		if err := TerraformApply(root, p.Reporter); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("terraform failed: %w", err))
			mu.Unlock()
		}
	})

	// Tailscale ACL push.
	wg.Go(func() {
		if err := TailscaleACLApply(p.Reporter); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("tailscale-acl failed: %w", err))
			mu.Unlock()
		}
	})

	wg.Wait()

	deployErr := errors.Join(errs...)
	duration := time.Since(deployStart).Milliseconds()
	status := "success"
	errMsg := ""
	if deployErr != nil {
		status = "failed"
		errMsg = deployErr.Error()
	}

	p.Reporter.FinishJob("deploy", FinishJobResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})

	return deployErr
}

func (p *Pipeline) fetchCode() error {
	start := time.Now()
	p.Reporter.StartJob("fetch", "fetch")
	p.Reporter.StartStream("fetch", "output")

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

	p.Reporter.FinishStream("fetch", "output", FinishStreamResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})

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
