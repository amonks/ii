package main

import (
	"fmt"
	"io"
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

	w := reporter.StreamWriter(jobName, "output")
	defer w.Close()

	start := time.Now()

	// Special case: if apps/ci itself is affected, use fly deploy
	// with the builder Dockerfile (which needs remote builder).
	if app == "ci" {
		err := deployCIBuilder(root, w)
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
	fmt.Fprintf(w, "=== compiling %s\n", app)
	compileStart := time.Now()
	binaryPath := filepath.Join(os.TempDir(), "bin", app, "app")
	os.MkdirAll(filepath.Dir(binaryPath), 0755)

	cmd := exec.Command("go", "build", "-ldflags", `-extldflags "-static"`, "-o", binaryPath, fmt.Sprintf("./apps/%s", app))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "GOOS=linux", "GOARCH=amd64", "MONKS_ROOT="+root)
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		errMsg := fmt.Sprintf("compile: %v", err)
		fmt.Fprintf(w, "=== compile failed: %s\n", errMsg)
		reporter.FinishJob(jobName, FinishJobResult{
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
		fmt.Fprintf(w, "=== warning: failed to pull base image, using empty: %v\n", err)
		baseImage = emptyImage()
	}

	img, err := oci.BuildAppImage(baseImage, binaryPath, files, imgCfg)
	if err != nil {
		errMsg := fmt.Sprintf("building image: %v", err)
		fmt.Fprintf(w, "=== image build failed: %s\n", errMsg)
		reporter.FinishJob(jobName, FinishJobResult{
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
		reporter.FinishJob(jobName, FinishJobResult{
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
		reporter.FinishJob(jobName, FinishJobResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("deploying %s: %w", app, err)
	}
	deployMs = time.Since(deployStart).Milliseconds()
	fmt.Fprintf(w, "=== deployed in %dms\n", deployMs)

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

	fmt.Fprintf(w, "=== done (%dms total)\n", totalDuration)
	return nil
}

func deployCIBuilder(root string, w io.Writer) error {
	fmt.Fprintf(w, "=== deploying CI builder (remote build)\n")
	cmd := exec.Command("fly", "deploy", "-c", "apps/ci/builder.fly.toml")
	cmd.Dir = root
	cmd.Stdout = w
	cmd.Stderr = w
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
