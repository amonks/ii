package main

import (
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// RunTests runs the generate and test tasks.
func RunTests(root string, reporter *Reporter) error {
	// Run generate first.
	if err := runTask(root, "generate", reporter); err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	// Run test.
	if err := runTask(root, "test", reporter); err != nil {
		return fmt.Errorf("test: %w", err)
	}

	return nil
}

func runTask(root, taskName string, reporter *Reporter) error {
	jobName := taskName
	reporter.StartJob(jobName, "task")

	start := time.Now()

	cmd := exec.Command("go", "tool", "run", taskName)
	cmd.Dir = root
	cmd.Env = append(cmd.Environ(), "MONKS_ROOT="+root)

	output, err := cmd.CombinedOutput()
	duration := time.Since(start).Milliseconds()

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
		slog.Error("task failed", "task", taskName, "error", err, "output", string(output))
	}

	reporter.FinishJob(jobName, FinishJobResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})

	if err != nil {
		return fmt.Errorf("%s: %w\n%s", taskName, err, string(output))
	}
	return nil
}
