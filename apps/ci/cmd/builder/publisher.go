package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"monks.co/pkg/ci/publish"
)

// PublishSubtrees publishes monorepo subtrees as public GitHub mirrors.
// Each mirror gets its own stream within the "deploy" job, running in
// parallel.
func PublishSubtrees(root string, reporter *Reporter) error {
	cfg, err := publish.LoadConfig(root)
	if err != nil {
		slog.Info("no publish config, skipping", "error", err)
		return nil
	}

	if len(cfg.Package) == 0 {
		slog.Info("no public packages configured, skipping publish")
		return nil
	}

	// Configure git to use gh for HTTPS authentication (uses GH_TOKEN env var).
	if setupErr := exec.Command("gh", "auth", "setup-git").Run(); setupErr != nil {
		slog.Warn("gh auth setup-git failed", "error", setupErr)
	}

	explicitPkgs, defaultMirrorDirs, err := publish.Analyze(root, cfg)
	if err != nil {
		return fmt.Errorf("publish analysis: %w", err)
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	// One goroutine per explicit mirror.
	for _, pkg := range explicitPkgs {
		wg.Go(func() {
			if err := publishExplicitMirror(root, pkg, reporter); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		})
	}

	// One goroutine for the default mirror (filter-repo).
	if len(defaultMirrorDirs) > 0 && cfg.DefaultMirror != "" {
		wg.Go(func() {
			if err := publishDefaultMirror(root, defaultMirrorDirs, cfg.DefaultMirror, reporter); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		})
	}

	wg.Wait()

	return errors.Join(errs...)
}

// streamName returns the stream name for a mirror. For explicit mirrors
// like "github.com/amonks/run", it uses the repo name ("publish-run").
// For the default mirror, it uses "publish-default".
func streamName(mirror string) string {
	return "publish-" + filepath.Base(mirror)
}

func publishExplicitMirror(root string, pkg publish.Package, reporter *Reporter) error {
	stream := streamName(pkg.Mirror)
	reporter.StartStream("deploy", stream)

	w := reporter.StreamWriter("deploy", stream)
	defer w.Close()

	start := time.Now()

	err := publish.PublishExplicitMirror(w, root, pkg)
	duration := time.Since(start).Milliseconds()

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
		fmt.Fprintf(w, "=== publish failed: %s\n", errMsg)
	} else {
		fmt.Fprintf(w, "=== done (%dms)\n", duration)
	}

	reporter.FinishStream("deploy", stream, FinishStreamResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})

	if err != nil {
		return fmt.Errorf("publish %s: %w", pkg.Dir, err)
	}
	return nil
}

func publishDefaultMirror(root string, dirs []string, mirror string, reporter *Reporter) error {
	stream := streamName(mirror)
	reporter.StartStream("deploy", stream)

	w := reporter.StreamWriter("deploy", stream)
	defer w.Close()

	start := time.Now()

	err := publish.PublishDefaultMirror(w, root, dirs, mirror)
	duration := time.Since(start).Milliseconds()

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
		fmt.Fprintf(w, "=== publish failed: %s\n", errMsg)
	} else {
		fmt.Fprintf(w, "=== done (%dms)\n", duration)
	}

	reporter.FinishStream("deploy", stream, FinishStreamResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})

	if err != nil {
		return fmt.Errorf("publish default mirror: %w", err)
	}
	return nil
}
