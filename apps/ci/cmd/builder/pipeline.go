package main

import (
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// Pipeline orchestrates the full CI pipeline.
type Pipeline struct {
	Config   *Config
	Reporter *Reporter
}

// Run executes the pipeline steps in order.
func (p *Pipeline) Run() error {
	root := p.Config.Root

	// Step 1: Fetch latest code.
	slog.Info("fetching latest code")
	if err := p.fetchCode(); err != nil {
		return fmt.Errorf("fetching code: %w", err)
	}

	// Step 2: Run generate + test.
	slog.Info("running tests")
	if err := RunTests(root, p.Reporter); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	// Step 3: Deploy affected apps.
	slog.Info("deploying affected apps")
	if err := DeployAffected(root, p.Config.HeadSHA, p.Config.BaseSHA, p.Config.FlyAPIToken, p.Config.BaseImageRef, p.Reporter); err != nil {
		return fmt.Errorf("deploy failed: %w", err)
	}

	// Step 4: Publish subtrees.
	slog.Info("publishing subtrees")
	if err := PublishSubtrees(root, p.Reporter); err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	// Step 5: Terraform apply.
	slog.Info("applying terraform")
	if err := TerraformApply(root, p.Reporter); err != nil {
		return fmt.Errorf("terraform failed: %w", err)
	}

	return nil
}

func (p *Pipeline) fetchCode() error {
	start := time.Now()
	p.Reporter.StartJob("fetch", "fetch")

	// Try jj first, fall back to git.
	err := p.tryJJFetch()
	if err != nil {
		err = p.tryGitFetch()
	}

	duration := time.Since(start).Milliseconds()
	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}

	p.Reporter.FinishJob("fetch", FinishJobResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})

	return err
}

func (p *Pipeline) tryJJFetch() error {
	cmd := exec.Command("jj", "git", "fetch")
	cmd.Dir = p.Config.Root
	return cmd.Run()
}

func (p *Pipeline) tryGitFetch() error {
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = p.Config.Root
	return cmd.Run()
}
