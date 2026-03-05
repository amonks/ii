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

// Run executes the pipeline steps in order. It returns a status string
// and an error. The status is "success", "failed", "restart-orchestrator",
// or "restart-builder-image".
func (p *Pipeline) Run(ctx context.Context) (string, error) {
	// Step 1: Fetch latest code (updates p.Config.Root to persistent volume).
	slog.Info("fetching latest code")
	if err := p.fetchCode(); err != nil {
		return "failed", fmt.Errorf("fetching code: %w", err)
	}

	root := p.Config.Root
	suffix := phaseSuffix(p.Config.Phase)

	// Step 2: Run generate + test.
	slog.Info("running tests", "phase", p.Config.Phase, "suffix", suffix)
	if err := RunTests(ctx, root, p.Reporter, suffix); err != nil {
		return "failed", fmt.Errorf("tests failed: %w", err)
	}

	// Step 3: Detect changes.
	analysis, err := DetectChanges(root, p.Config.BaseSHA)
	if err != nil {
		return "failed", fmt.Errorf("change detection: %w", err)
	}

	// Step 4: Phase-dependent infrastructure checks.
	switch p.Config.Phase {
	case "initial":
		if analysis.CIAffected {
			if err := p.deployOrchestrator(root, analysis); err != nil {
				return "failed", fmt.Errorf("deploying orchestrator: %w", err)
			}
			return "restart-orchestrator", nil
		}
		if analysis.BuilderAffected {
			if err := p.rebuildBuilderImage(root); err != nil {
				return "failed", fmt.Errorf("rebuilding builder image: %w", err)
			}
			return "restart-builder-image", nil
		}
	case "post-orchestrator":
		if analysis.BuilderAffected {
			if err := p.rebuildBuilderImage(root); err != nil {
				return "failed", fmt.Errorf("rebuilding builder image: %w", err)
			}
			return "restart-builder-image", nil
		}
	case "post-builder":
		// No infrastructure checks needed.
	}

	// Step 5: Deploy apps (excluding ci, with builderAffected forced off).
	if err := p.runDeploy(ctx, root, analysis); err != nil {
		return "failed", err
	}

	return "success", nil
}

// phaseSuffix returns the job name suffix for the given phase.
func phaseSuffix(phase string) string {
	switch phase {
	case "post-orchestrator":
		return "-2"
	case "post-builder":
		return "-3"
	default:
		return ""
	}
}

// deployOrchestrator deploys the CI orchestrator app as its own job.
func (p *Pipeline) deployOrchestrator(root string, analysis *ChangeAnalysis) error {
	const jobName = "orchestrator-deploy"
	p.Reporter.StartJob(jobName, "deploy")
	start := time.Now()

	err := deployAppFunc(root, "ci", p.Config.HeadSHA, p.Config.FlyAPIToken, p.Config.BaseImageRef, analysis.Cfg, p.Reporter, jobName)

	duration := time.Since(start).Milliseconds()
	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}
	p.Reporter.FinishJob(jobName, FinishJobResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})
	return err
}

// rebuildBuilderImage rebuilds the builder Docker image as its own job.
func (p *Pipeline) rebuildBuilderImage(root string) error {
	const jobName = "builder-image-rebuild"
	p.Reporter.StartJob(jobName, "deploy")
	start := time.Now()

	err := rebuildImageFunc(root, "ci-builder", "apps/ci/builder.fly.toml", p.Reporter, jobName)

	duration := time.Since(start).Milliseconds()
	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}
	p.Reporter.FinishJob(jobName, FinishJobResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})
	return err
}

// runDeploy runs the deploy phase: analysis stream + concurrent app
// deploys, publish, terraform, and tailscale ACL. CI is excluded from
// affected apps, and builderAffected is forced off (handled in pre-flight).
func (p *Pipeline) runDeploy(ctx context.Context, root string, analysis *ChangeAnalysis) error {
	p.Reporter.StartJob("deploy", "deploy")
	deployStart := time.Now()

	// Report analysis results in a stream.
	if err := p.reportAnalysis(analysis); err != nil {
		duration := time.Since(deployStart).Milliseconds()
		p.Reporter.FinishJob("deploy", FinishJobResult{
			Status:     "failed",
			DurationMs: duration,
			Error:      err.Error(),
		})
		return fmt.Errorf("deploy analysis: %w", err)
	}

	// Filter out ci from affected and force builderAffected off.
	filtered := make([]string, 0, len(analysis.Affected))
	for _, app := range analysis.Affected {
		if app != "ci" {
			filtered = append(filtered, app)
		}
	}
	deployAnalysisData := &deployAnalysis{
		affected:        filtered,
		builderAffected: false,
		baseAffected:    analysis.BaseAffected,
		cfg:             analysis.Cfg,
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	// Deploy affected apps and rebuild base image.
	wg.Go(func() {
		if err := DeployAnalyzed(deployAnalysisData, root, p.Config.HeadSHA, p.Config.FlyAPIToken, p.Config.BaseImageRef, p.Reporter); err != nil {
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

// reportAnalysis writes the change detection results to an "analysis"
// stream within the "deploy" job.
func (p *Pipeline) reportAnalysis(analysis *ChangeAnalysis) error {
	p.Reporter.StartStream("deploy", "analysis")
	w := p.Reporter.StreamWriter("deploy", "analysis")
	defer w.Close()

	start := time.Now()

	affectedSet := make(map[string]bool, len(analysis.Affected))
	for _, a := range analysis.Affected {
		affectedSet[a] = true
	}

	for _, app := range analysis.Affected {
		fmt.Fprintf(w, "  %s: affected\n", app)
	}
	if analysis.CIAffected {
		fmt.Fprintf(w, "  ci: affected (handled in pre-flight)\n")
	}
	if analysis.BuilderAffected {
		fmt.Fprintf(w, "  ci-builder: rebuild needed (handled in pre-flight)\n")
	}
	if analysis.BaseAffected {
		fmt.Fprintf(w, "  ci-base: rebuild needed\n")
	}

	fmt.Fprintf(w, "=== %d apps affected\n", len(analysis.Affected))

	slog.Info("deploy analysis",
		"affected", len(analysis.Affected),
		"ci_affected", analysis.CIAffected,
		"builder_rebuild", analysis.BuilderAffected,
		"base_rebuild", analysis.BaseAffected)

	p.Reporter.FinishStream("deploy", "analysis", FinishStreamResult{
		Status:     "success",
		DurationMs: time.Since(start).Milliseconds(),
	})

	return nil
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
