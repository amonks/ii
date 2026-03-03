package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"monks.co/pkg/ci/changedetect"
	"monks.co/pkg/depgraph"
	"monks.co/pkg/oci"
)

// DeployAffected builds and deploys all apps affected by the changes.
func DeployAffected(root, headSHA, baseSHA, flyToken, baseImageRef string, reporter *Reporter) error {
	apps, err := changedetect.LoadFlyApps(root)
	if err != nil {
		return fmt.Errorf("loading fly apps: %w", err)
	}

	changed, err := changedetect.ChangedFiles(root, baseSHA)
	if err != nil {
		return fmt.Errorf("getting changed files: %w", err)
	}

	graph, err := depgraph.BuildDepGraph(root)
	if err != nil {
		return fmt.Errorf("building dep graph: %w", err)
	}

	affected := changedetect.AffectedApps(apps, changed, graph)

	if len(affected) == 0 {
		slog.Info("no apps affected by changes")
		return nil
	}

	slog.Info("affected apps", "apps", strings.Join(affected, ", "))

	cfg, err := changedetect.LoadFlyAppsConfig(root)
	if err != nil {
		return fmt.Errorf("loading fly apps config: %w", err)
	}

	for _, app := range affected {
		if err := deployApp(root, app, headSHA, flyToken, baseImageRef, cfg, reporter); err != nil {
			return fmt.Errorf("deploying %s: %w", app, err)
		}
	}

	return nil
}

func deployApp(root, app, sha, flyToken, baseImageRef string, cfg *changedetect.FlyAppsConfig, reporter *Reporter) error {
	jobName := "deploy-" + app
	reporter.StartJob(jobName, "deploy")

	start := time.Now()

	// Special case: if apps/ci itself is affected, use fly deploy
	// with the builder Dockerfile (which needs remote builder).
	if app == "ci" {
		err := deployCIBuilder(root)
		duration := time.Since(start).Milliseconds()
		status := "success"
		errMsg := ""
		if err != nil {
			status = "failed"
			errMsg = err.Error()
		}
		reporter.FinishJob(jobName, FinishJobResult{
			Status:     status,
			DurationMs: duration,
			Error:      errMsg,
		})
		return err
	}

	var compileMs, pushMs, deployMs, binaryBytes, imageBytes int64
	imageRef := fmt.Sprintf("registry.fly.io/monks-%s:%s", app, sha)

	// Step 1: Compile the binary.
	compileStart := time.Now()
	binaryPath := filepath.Join(os.TempDir(), "bin", app)
	os.MkdirAll(filepath.Dir(binaryPath), 0755)

	cmd := exec.Command("go", "build", "-o", binaryPath, fmt.Sprintf("./apps/%s", app))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "MONKS_ROOT="+root)
	if output, err := cmd.CombinedOutput(); err != nil {
		reporter.FinishJob(jobName, FinishJobResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("compile: %v\n%s", err, string(output)),
		})
		return fmt.Errorf("compiling %s: %w", app, err)
	}
	compileMs = time.Since(compileStart).Milliseconds()

	if info, err := os.Stat(binaryPath); err == nil {
		binaryBytes = info.Size()
	}

	// Step 2: Build OCI image.
	appCfg := cfg.Apps[app]

	files := map[string]string{}
	for _, f := range appCfg.Files {
		files[filepath.Join(root, f)] = filepath.Join("/app", f)
	}

	entrypoint := appCfg.Cmd
	if len(entrypoint) == 0 {
		entrypoint = []string{"/app/bin/app"}
	}

	envVars := []string{
		"MONKS_APP_NAME=" + app,
		"MONKS_ROOT=/app",
		"TSNET_FORCE_LOGIN=1",
	}
	if appCfg.Volume != "" {
		envVars = append(envVars, "MONKS_DATA=/data")
	} else {
		envVars = append(envVars, "MONKS_DATA=/tmp")
	}

	imgCfg := oci.ImageConfig{
		Cmd:     entrypoint,
		Env:     envVars,
		WorkDir: fmt.Sprintf("/app/apps/%s", app),
	}

	var baseImage v1.Image
	baseImage, err := remote.Image(
		mustParseRef(baseImageRef),
		oci.FlyAuthOption(flyToken),
	)
	if err != nil {
		slog.Warn("failed to pull base image, using empty", "error", err)
		baseImage = emptyImage()
	}

	img, err := oci.BuildAppImage(baseImage, binaryPath, files, imgCfg)
	if err != nil {
		reporter.FinishJob(jobName, FinishJobResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("building image: %v", err),
		})
		return fmt.Errorf("building image for %s: %w", app, err)
	}

	// Step 3: Push to registry.
	pushStart := time.Now()
	if err := oci.Push(img, imageRef, oci.FlyAuthOption(flyToken)); err != nil {
		reporter.FinishJob(jobName, FinishJobResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("pushing image: %v", err),
		})
		return fmt.Errorf("pushing image for %s: %w", app, err)
	}
	pushMs = time.Since(pushStart).Milliseconds()

	// Step 4: Deploy via flyctl.
	deployStart := time.Now()
	tomlPath := filepath.Join("apps", app, "fly.toml")
	deployCmd := exec.Command("fly", "deploy",
		"--image", imageRef,
		"-c", tomlPath,
	)
	deployCmd.Dir = root
	if output, err := deployCmd.CombinedOutput(); err != nil {
		reporter.FinishJob(jobName, FinishJobResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("fly deploy: %v\n%s", err, string(output)),
		})
		return fmt.Errorf("deploying %s: %w", app, err)
	}
	deployMs = time.Since(deployStart).Milliseconds()

	totalDuration := time.Since(start).Milliseconds()

	reporter.FinishJob(jobName, FinishJobResult{
		Status:     "success",
		DurationMs: totalDuration,
		Deploy: &DeployData{
			App:         app,
			ImageRef:    imageRef,
			BinaryBytes: binaryBytes,
			ImageBytes:  imageBytes,
			CompileMs:   compileMs,
			PushMs:      pushMs,
			DeployMs:    deployMs,
		},
	})

	reporter.RecordDeployment(app, sha, imageRef, binaryBytes)

	slog.Info("deployed", "app", app, "sha", sha[:7], "duration_ms", totalDuration)
	return nil
}

func deployCIBuilder(root string) error {
	cmd := exec.Command("fly", "deploy", "-c", "apps/ci/builder.fly.toml")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func mustParseRef(s string) name.Reference {
	ref, err := name.ParseReference(s)
	if err != nil {
		panic(err)
	}
	return ref
}

func emptyImage() v1.Image {
	return empty.Image
}
