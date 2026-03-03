package main

import (
	"fmt"
	"log/slog"
	"time"

	"monks.co/pkg/ci/publish"
)

// PublishSubtrees publishes monorepo subtrees as public GitHub mirrors.
func PublishSubtrees(root string, reporter *Reporter) error {
	reporter.StartJob("publish", "publish")
	start := time.Now()

	cfg, err := publish.LoadConfig(root)
	if err != nil {
		// Config might not exist yet.
		slog.Info("no publish config, skipping", "error", err)
		reporter.FinishJob("publish", FinishJobResult{
			Status:     "success",
			DurationMs: time.Since(start).Milliseconds(),
		})
		return nil
	}

	if len(cfg.Package) == 0 {
		slog.Info("no public packages configured, skipping publish")
		reporter.FinishJob("publish", FinishJobResult{
			Status:     "success",
			DurationMs: time.Since(start).Milliseconds(),
		})
		return nil
	}

	err = publish.Run(root, cfg, false)
	duration := time.Since(start).Milliseconds()

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
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
