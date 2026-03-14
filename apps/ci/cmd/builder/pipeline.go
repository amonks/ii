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

// fetchCodeFunc can be overridden in tests to skip real git operations.
var fetchCodeFunc func(p *Pipeline) error

// Run executes the pipeline steps in order. It returns a status string
// and an error. The status is "success", "failed", "restart-orchestrator",
// or "restart-builder-image".
func (p *Pipeline) Run(ctx context.Context) (string, error) {
	// Step 1: Fetch latest code (updates p.Config.Root to persistent volume).
	slog.Info("fetching latest code")
	fetchFn := p.fetchCode
	if fetchCodeFunc != nil {
		fetchFn = func() error { return fetchCodeFunc(p) }
	}
	if err := fetchFn(); err != nil {
		return "failed", fmt.Errorf("fetching code: %w", err)
	}

	root := p.Config.Root

	// Step 2: In the initial phase, check if the builder image needs
	// rebuilding before running tests. This ensures that when a commit
	// changes both the Dockerfile and code that depends on the new
	// builder environment, the image is rebuilt first and tests run on
	// the new builder in the post-builder phase.
	if p.Config.Phase == "initial" {
		analysis, err := DetectChanges(root, p.Config.BaseSHA)
		if err != nil {
			return "failed", fmt.Errorf("change detection: %w", err)
		}
		if analysis.BuilderAffected {
			if err := p.rebuildBuilderImage(root); err != nil {
				return "failed", fmt.Errorf("rebuilding builder image: %w", err)
			}
			return "restart-builder-image", nil
		}
	}

	// Step 3: Generate + test.
	suffix := phaseSuffix(p.Config.Phase)
	slog.Info("running tests", "phase", p.Config.Phase, "suffix", suffix)
	if err := RunTests(ctx, root, p.Reporter, suffix); err != nil {
		return "failed", fmt.Errorf("tests failed: %w", err)
	}

	// Step 4: Detect changes for deploy decisions. In the initial phase
	// this re-runs change detection (cheap: git diff + dep graph walk),
	// but now we need the full analysis for CI deploy and app deploy.
	analysis, err := DetectChanges(root, p.Config.BaseSHA)
	if err != nil {
		return "failed", fmt.Errorf("change detection: %w", err)
	}

	// Step 5: Phase-dependent infrastructure checks.
	switch p.Config.Phase {
	case "initial":
		// Builder was already handled above. Check CI orchestrator.
		if analysis.CIAffected {
			if err := p.deployOrchestrator(root, analysis); err != nil {
				return "failed", fmt.Errorf("deploying orchestrator: %w", err)
			}
			return "restart-orchestrator", nil
		}
	case "post-orchestrator":
		// Builder already handled in initial phase. No more infra checks.
	case "post-builder":
		// No infrastructure checks needed.
	}

	// Step 6: Deploy apps (excluding ci, with builderAffected forced off).
	if err := p.runDeploy(ctx, root, analysis); err != nil {
		return "failed", err
	}

	return "success", nil
}

// phaseSuffix returns the job name suffix for the given phase.
func phaseSuffix(phase string) string {
	switch phase {
	case "post-orchestrator":
		return "-post-orchestrator"
	case "post-builder":
		return "-post-builder"
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

// runDeployFunc can be overridden in tests to skip real deploy operations.
var runDeployFunc func(ctx context.Context, p *Pipeline, root string, analysis *ChangeAnalysis) error

// runDeploy runs the deploy phase: analysis stream + concurrent app
// deploys, publish, terraform, and tailscale ACL. CI is excluded from
// affected apps, and builderAffected is forced off (handled in pre-flight).
func (p *Pipeline) runDeploy(ctx context.Context, root string, analysis *ChangeAnalysis) error {
	if runDeployFunc != nil {
		return runDeployFunc(ctx, p, root, analysis)
	}
	return p.runDeployReal(ctx, root, analysis)
}

func (p *Pipeline) runDeployReal(ctx context.Context, root string, analysis *ChangeAnalysis) error {
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
