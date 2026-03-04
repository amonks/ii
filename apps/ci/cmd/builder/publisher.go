package main

import (
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"monks.co/pkg/ci/publish"
)

// PublishSubtrees publishes monorepo subtrees as public GitHub mirrors.
func PublishSubtrees(root string, reporter *Reporter) error {
	reporter.StartJob("publish", "publish")

	w := reporter.StreamWriter("publish", "output")
	defer w.Close()

	start := time.Now()

	cfg, err := publish.LoadConfig(root)
	if err != nil {
		// Config might not exist yet.
		slog.Info("no publish config, skipping", "error", err)
		fmt.Fprintf(w, "no publish config, skipping: %v\n", err)
		reporter.FinishJob("publish", FinishJobResult{
			Status:     "success",
			DurationMs: time.Since(start).Milliseconds(),
		})
		return nil
	}

	if len(cfg.Package) == 0 {
		slog.Info("no public packages configured, skipping publish")
		fmt.Fprintf(w, "no public packages configured, skipping\n")
		reporter.FinishJob("publish", FinishJobResult{
			Status:     "success",
			DurationMs: time.Since(start).Milliseconds(),
		})
		return nil
	}

	// Configure git to use gh for HTTPS authentication (uses GH_TOKEN env var).
	if setupErr := exec.Command("gh", "auth", "setup-git").Run(); setupErr != nil {
		slog.Warn("gh auth setup-git failed", "error", setupErr)
	}

	err = publish.Run(w, root, cfg, false)
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

	reporter.FinishJob("publish", FinishJobResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})

	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}
