package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/amonks/run/runner"
	"github.com/amonks/run/taskfile"
)

// RunTests runs the generate and test tasks using the run library
// programmatically, streaming per-task output to the orchestrator.
func RunTests(ctx context.Context, root string, reporter *Reporter) error {
	if err := runTask(ctx, root, "generate", reporter); err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if err := runTask(ctx, root, "test", reporter); err != nil {
		return fmt.Errorf("test: %w", err)
	}
	return nil
}

func runTask(ctx context.Context, root, taskName string, reporter *Reporter) error {
	reporter.StartJob(taskName, "task")
	start := time.Now()

	tasks, err := taskfile.Load(root)
	if err != nil {
		errMsg := fmt.Sprintf("loading taskfile: %v", err)
		reporter.FinishJob(taskName, FinishJobResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("loading taskfile: %w", err)
	}

	mw := &streamMultiWriter{
		reporter: reporter,
		jobName:  taskName,
		writers:  make(map[string]*StreamWriter),
	}

	run, err := runner.New(runner.RunTypeShort, root, tasks, taskName, mw)
	if err != nil {
		errMsg := fmt.Sprintf("creating runner: %v", err)
		reporter.FinishJob(taskName, FinishJobResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		mw.CloseAll()
		return fmt.Errorf("creating runner: %w", err)
	}

	// Start streams for each non-internal task ID.
	ids := run.IDs()
	var streamIDs []string
	for _, id := range ids {
		if strings.HasPrefix(id, "@") {
			continue
		}
		streamIDs = append(streamIDs, id)
		reporter.StartStream(taskName, id)
	}

	err = run.Start(ctx)
	duration := time.Since(start).Milliseconds()

	mw.CloseAll()

	// Finish streams with per-task status from the runner.
	for _, id := range streamIDs {
		ts := run.TaskStatus(id)
		streamStatus := "skipped"
		switch ts {
		case runner.TaskStatusDone:
			streamStatus = "success"
		case runner.TaskStatusFailed:
			streamStatus = "failed"
		case runner.TaskStatusRunning:
			streamStatus = "success" // was still running when run ended normally
		}
		reporter.FinishStream(taskName, id, FinishStreamResult{
			Status: streamStatus,
		})
	}

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}

	reporter.FinishJob(taskName, FinishJobResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})

	if err != nil {
		return fmt.Errorf("%s: %w", taskName, err)
	}
	return nil
}

// streamMultiWriter implements runner.MultiWriter, returning a StreamWriter
// per task ID so each task's output is streamed separately.
type streamMultiWriter struct {
	reporter *Reporter
	jobName  string
	writers  map[string]*StreamWriter
}

func (m *streamMultiWriter) Writer(id string) io.Writer {
	sw := m.reporter.StreamWriter(m.jobName, id)
	m.writers[id] = sw
	return sw
}

func (m *streamMultiWriter) CloseAll() {
	for _, sw := range m.writers {
		sw.Close()
	}
}
