package main

import (
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"monks.co/pkg/ci/publish"
)

// PublishSubtrees publishes monorepo subtrees as public GitHub mirrors
// using the unified publish flow (versioning, go.mod rewriting, tagging).
func PublishSubtrees(root string, reporter *Reporter) error {
	cfg, err := publish.LoadConfig(root)
	if err != nil {
		return fmt.Errorf("loading publish config: %w", err)
	}

	if len(cfg.Package) == 0 {
		slog.Info("no public packages configured, skipping publish")
		return nil
	}

	// Configure git to use gh for HTTPS authentication (uses GH_TOKEN env var).
	if setupErr := exec.Command("gh", "auth", "setup-git").Run(); setupErr != nil {
		return fmt.Errorf("gh auth setup-git: %w", setupErr)
	}

	stream := "publish"
	reporter.StartStream("deploy", stream)

	w := reporter.StreamWriter("deploy", stream)
	defer w.Close()

	start := time.Now()

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

	if fsErr := reporter.FinishStream("deploy", stream, FinishStreamResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	}); fsErr != nil {
		slog.Warn("failed to finish stream", "stream", stream, "error", fsErr)
	}

	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}
