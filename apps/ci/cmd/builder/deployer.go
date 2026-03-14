package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
"github.com/google/go-containerregistry/pkg/v1/remote"
	"slices"

	"monks.co/pkg/ci/changedetect"
	"monks.co/pkg/config"
	"monks.co/pkg/depgraph"
	"monks.co/pkg/oci"
)

// deployAppFunc is the function used to deploy a single app. It can be
// replaced in tests.
var deployAppFunc = deployApp

// rebuildImageFunc is the function used to rebuild a CI image. It can be
// replaced in tests.
var rebuildImageFunc = rebuildImage

// ChangeAnalysis holds the results of change detection analysis.
type ChangeAnalysis struct {
	Affected        []string
	CIAffected      bool
	BuilderAffected bool
	BaseAffected    bool
	Cfg             *config.AppsConfig
}

// detectChangesFunc is the function used to detect changes. It can be
// replaced in tests.
var detectChangesFunc = detectChanges

// DetectChanges runs change detection and returns what's affected.
func DetectChanges(root, baseSHA string) (*ChangeAnalysis, error) {
	return detectChangesFunc(root, baseSHA)
}

func detectChanges(root, baseSHA string) (*ChangeAnalysis, error) {
	apps, err := changedetect.LoadFlyApps(root)
	if err != nil {
		return nil, fmt.Errorf("loading fly apps: %w", err)
	}

	changed, err := changedetect.ChangedFiles(root, baseSHA)
	if err != nil {
		return nil, fmt.Errorf("getting changed files: %w", err)
	}

	resolveDeps := func(pkgPath string) ([]string, error) {
		return depgraph.PackageDeps(root, pkgPath)
	}

	affected, err := changedetect.AffectedApps(apps, changed, resolveDeps)
	if err != nil {
		return nil, fmt.Errorf("computing affected apps: %w", err)
	}

	builderAffected, err := changedetect.IsImageAffected(changed, "apps/ci/builder.Dockerfile", resolveDeps, "apps/ci/cmd/builder")
	if err != nil {
		return nil, fmt.Errorf("checking builder image: %w", err)
	}

	baseAffected, err := changedetect.IsImageAffected(changed, "apps/ci/base.Dockerfile", nil, "")
	if err != nil {
		return nil, fmt.Errorf("checking base image: %w", err)
	}

	cfg, err := changedetect.LoadFlyAppsConfig(root)
	if err != nil {
		return nil, fmt.Errorf("loading fly apps config: %w", err)
	}

	// Determine if the CI orchestrator app itself is affected.
	ciAffected := slices.Contains(affected, "ci")

	return &ChangeAnalysis{
		Affected:        affected,
		CIAffected:      ciAffected,
		BuilderAffected: builderAffected,
		BaseAffected:    baseAffected,
		Cfg:             cfg,
	}, nil
}

// deployAnalysis holds the results of change detection analysis.
type deployAnalysis struct {
	affected        []string
	builderAffected bool
	baseAffected    bool
	cfg             *config.AppsConfig
}

// AnalyzeDeploy runs change detection and reports results in an
// "analysis" stream within the "deploy" job. Returns what needs
// deploying/rebuilding.
func AnalyzeDeploy(root, baseSHA string, reporter *Reporter) (*deployAnalysis, error) {
	reporter.StartStream("deploy", "analysis")
	w := reporter.StreamWriter("deploy", "analysis")
	defer w.Close()

	start := time.Now()

	apps, err := changedetect.LoadFlyApps(root)
	if err != nil {
		errMsg := fmt.Sprintf("loading fly apps: %v", err)
		fmt.Fprintf(w, "=== %s\n", errMsg)
		reporter.FinishStream("deploy", "analysis", FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return nil, fmt.Errorf("loading fly apps: %w", err)
	}

	if len(apps) == 0 {
		fmt.Fprintf(w, "no fly apps configured\n")
		slog.Info("no fly apps configured")
	}

	changed, err := changedetect.ChangedFiles(root, baseSHA)
	if err != nil {
		errMsg := fmt.Sprintf("getting changed files: %v", err)
		fmt.Fprintf(w, "=== %s\n", errMsg)
		reporter.FinishStream("deploy", "analysis", FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return nil, fmt.Errorf("getting changed files: %w", err)
	}

	resolveDeps := func(pkgPath string) ([]string, error) {
		return depgraph.PackageDeps(root, pkgPath)
	}

	affected, err := changedetect.AffectedApps(apps, changed, resolveDeps)
	if err != nil {
		errMsg := fmt.Sprintf("computing affected apps: %v", err)
		fmt.Fprintf(w, "=== %s\n", errMsg)
		reporter.FinishStream("deploy", "analysis", FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return nil, fmt.Errorf("computing affected apps: %w", err)
	}

	affectedSet := make(map[string]bool, len(affected))
	for _, a := range affected {
		affectedSet[a] = true
	}

	// Determine which images need rebuilding.
	builderAffected, err := changedetect.IsImageAffected(changed, "apps/ci/builder.Dockerfile", resolveDeps, "apps/ci/cmd/builder")
	if err != nil {
		errMsg := fmt.Sprintf("checking builder image: %v", err)
		fmt.Fprintf(w, "=== %s\n", errMsg)
		reporter.FinishStream("deploy", "analysis", FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return nil, fmt.Errorf("checking builder image: %w", err)
	}

	baseAffected, err := changedetect.IsImageAffected(changed, "apps/ci/base.Dockerfile", nil, "")
	if err != nil {
		errMsg := fmt.Sprintf("checking base image: %v", err)
		fmt.Fprintf(w, "=== %s\n", errMsg)
		reporter.FinishStream("deploy", "analysis", FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return nil, fmt.Errorf("checking base image: %w", err)
	}

	cfg, err := changedetect.LoadFlyAppsConfig(root)
	if err != nil {
		errMsg := fmt.Sprintf("loading fly apps config: %v", err)
		fmt.Fprintf(w, "=== %s\n", errMsg)
		reporter.FinishStream("deploy", "analysis", FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return nil, fmt.Errorf("loading fly apps config: %w", err)
	}

	// Report results.
	slog.Info("deploy analysis", "total_apps", len(apps), "affected", len(affected),
		"builder_rebuild", builderAffected, "base_rebuild", baseAffected)

	for _, app := range apps {
		if affectedSet[app] {
			fmt.Fprintf(w, "  %s: affected\n", app)
		} else {
			fmt.Fprintf(w, "  %s: skipped\n", app)
		}
	}
	if builderAffected {
		fmt.Fprintf(w, "  ci-builder: rebuild needed\n")
	}
	if baseAffected {
		fmt.Fprintf(w, "  ci-base: rebuild needed\n")
	}

	fmt.Fprintf(w, "=== %d/%d apps affected\n", len(affected), len(apps))

	reporter.FinishStream("deploy", "analysis", FinishStreamResult{
		Status:     "success",
		DurationMs: time.Since(start).Milliseconds(),
	})

	return &deployAnalysis{
		affected:        affected,
		builderAffected: builderAffected,
		baseAffected:    baseAffected,
		cfg:             cfg,
	}, nil
}

// DeployAnalyzed deploys affected apps and rebuilds images based on
// analysis results. Each app and image rebuild gets its own stream
// within the "deploy" job. The caller is responsible for starting
// and finishing the job.
func DeployAnalyzed(analysis *deployAnalysis, root, headSHA, flyToken, baseImageRef string, reporter *Reporter) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for _, app := range analysis.affected {
		wg.Go(func() {
			if err := deployAppFunc(root, app, headSHA, flyToken, baseImageRef, analysis.cfg, reporter, "deploy"); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("deploying %s: %w", app, err))
				mu.Unlock()
			}
		})
	}

	if analysis.builderAffected {
		wg.Go(func() {
			if err := rebuildImageFunc(root, "ci-builder", "apps/ci/builder.fly.toml", reporter, "deploy"); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("rebuilding ci-builder: %w", err))
				mu.Unlock()
			}
		})
	}

	if analysis.baseAffected {
		wg.Go(func() {
			if err := rebuildImageFunc(root, "ci-base", "apps/ci/base.fly.toml", reporter, "deploy"); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("rebuilding ci-base: %w", err))
				mu.Unlock()
			}
		})
	}

	wg.Wait()

	return errors.Join(errs...)
}

func deployApp(root, app, sha, flyToken, baseImageRef string, cfg *config.AppsConfig, reporter *Reporter, jobName string) error {
	reporter.StartStream(jobName, app)

	w := reporter.StreamWriter(jobName, app)
	defer w.Close()

	start := time.Now()

	var compileMs, pushMs, deployMs, binaryBytes, imageBytes int64
	imageRef := fmt.Sprintf("registry.fly.io/monks-%s:%s", app, sha)

	// Step 1: Compile the binary.
	fmt.Fprintf(w, "=== compiling %s\n", app)
	compileStart := time.Now()
	binaryPath := filepath.Join(os.TempDir(), "bin", app, "app")
	os.MkdirAll(filepath.Dir(binaryPath), 0755)

	cmd := exec.Command("go", "build", "-o", binaryPath, fmt.Sprintf("./apps/%s", app))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "MONKS_ROOT="+root)
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		errMsg := fmt.Sprintf("compile: %v", err)
		fmt.Fprintf(w, "=== compile failed: %s\n", errMsg)
		reporter.FinishStream(jobName, app, FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("compiling %s: %w", app, err)
	}
	compileMs = time.Since(compileStart).Milliseconds()
	fmt.Fprintf(w, "=== compiled in %dms\n", compileMs)

	if info, err := os.Stat(binaryPath); err == nil {
		binaryBytes = info.Size()
	}

	// Step 2: Build OCI image.
	fmt.Fprintf(w, "=== building OCI image\n")
	appCfg := cfg.Apps[app]

	files := map[string]string{}
	for _, f := range appCfg.Files {
		files[filepath.Join(root, f)] = filepath.Join("/app", f)
	}

	entrypoint := appCfg.Cmd
	if len(entrypoint) == 0 {
		entrypoint = []string{"/app/app"}
	}

	envVars := []string{
		"MONKS_APP_NAME=" + app,
		"MONKS_ROOT=/app",
		"MONKS_DATA=/data",
		"TSNET_FORCE_LOGIN=1",
	}

	imgCfg := oci.ImageConfig{
		Cmd:     entrypoint,
		Env:     envVars,
		WorkDir: fmt.Sprintf("/app/apps/%s", app),
	}

	baseImage, err := remote.Image(
		mustParseRef(baseImageRef),
		oci.FlyAuthOption(flyToken),
	)
	if err != nil {
		errMsg := fmt.Sprintf("pulling base image %s: %v", baseImageRef, err)
		fmt.Fprintf(w, "=== %s\n", errMsg)
		reporter.FinishStream(jobName, app, FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("pulling base image for %s: %w", app, err)
	}

	img, err := oci.BuildAppImage(baseImage, binaryPath, files, imgCfg)
	if err != nil {
		errMsg := fmt.Sprintf("building image: %v", err)
		fmt.Fprintf(w, "=== image build failed: %s\n", errMsg)
		reporter.FinishStream(jobName, app, FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("building image for %s: %w", app, err)
	}

	// Step 3: Push to registry.
	fmt.Fprintf(w, "=== pushing %s\n", imageRef)
	pushStart := time.Now()
	if err := oci.Push(img, imageRef, oci.FlyAuthOption(flyToken)); err != nil {
		errMsg := fmt.Sprintf("pushing image: %v", err)
		fmt.Fprintf(w, "=== push failed: %s\n", errMsg)
		reporter.FinishStream(jobName, app, FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("pushing image for %s: %w", app, err)
	}
	pushMs = time.Since(pushStart).Milliseconds()
	fmt.Fprintf(w, "=== pushed in %dms\n", pushMs)

	// Step 4: Deploy via flyctl.
	fmt.Fprintf(w, "=== deploying %s\n", app)
	deployStart := time.Now()
	tomlPath := filepath.Join("apps", app, "fly.toml")
	deployCmd := exec.Command("fly", "deploy",
		"--image", imageRef,
		"-c", tomlPath,
	)
	deployCmd.Dir = root
	deployCmd.Stdout = w
	deployCmd.Stderr = w
	if err := deployCmd.Run(); err != nil {
		errMsg := fmt.Sprintf("fly deploy: %v", err)
		fmt.Fprintf(w, "=== deploy failed: %s\n", errMsg)
		reporter.FinishStream(jobName, app, FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("deploying %s: %w", app, err)
	}
	deployMs = time.Since(deployStart).Milliseconds()
	fmt.Fprintf(w, "=== deployed in %dms\n", deployMs)

	totalDuration := time.Since(start).Milliseconds()

	reporter.FinishStream(jobName, app, FinishStreamResult{
		Status:     "success",
		DurationMs: totalDuration,
	})

	reporter.AddDeployResult(DeployResult{
		App:         app,
		ImageRef:    imageRef,
		BinaryBytes: binaryBytes,
		ImageBytes:  imageBytes,
		CompileMs:   compileMs,
		PushMs:      pushMs,
		DeployMs:    deployMs,
	})

	reporter.RecordDeployment(app, sha, imageRef, binaryBytes)

	fmt.Fprintf(w, "=== done (%dms total)\n", totalDuration)
	return nil
}

func rebuildImage(root, name, tomlPath string, reporter *Reporter, jobName string) error {
	reporter.StartStream(jobName, name)
	w := reporter.StreamWriter(jobName, name)
	defer w.Close()

	start := time.Now()

	fmt.Fprintf(w, "=== rebuilding %s image\n", name)
	cmd := exec.Command("fly", "deploy", "-c", tomlPath, "--build-only", "--push")
	cmd.Dir = root
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		errMsg := fmt.Sprintf("rebuilding %s: %v", name, err)
		fmt.Fprintf(w, "=== rebuild failed: %s\n", errMsg)
		reporter.FinishStream(jobName, name, FinishStreamResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("rebuilding %s: %w", name, err)
	}

	duration := time.Since(start).Milliseconds()
	fmt.Fprintf(w, "=== rebuilt in %dms\n", duration)
	reporter.FinishStream(jobName, name, FinishStreamResult{
		Status:     "success",
		DurationMs: duration,
	})
	return nil
}

func mustParseRef(s string) name.Reference {
	ref, err := name.ParseReference(s)
	if err != nil {
		panic(err)
	}
	return ref
}

