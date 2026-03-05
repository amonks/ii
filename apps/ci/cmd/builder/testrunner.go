package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"monks.co/run/runner"
	"monks.co/run/taskfile"
)

// encodeStreamName makes a task ID safe for use as a single URL path segment
// by replacing "/" with "~".
func encodeStreamName(name string) string {
	return strings.ReplaceAll(name, "/", "~")
}

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
	// Encode IDs to be URL-safe (task IDs like "apps/ci/build-js"
	// contain slashes which break HTTP path routing).
	ids := run.IDs()
	type streamMapping struct {
		taskID     string
		streamName string
	}
	var streams []streamMapping
	for _, id := range ids {
		if strings.HasPrefix(id, "@") {
			continue
		}
		sn := encodeStreamName(id)
		streams = append(streams, streamMapping{taskID: id, streamName: sn})
		reporter.StartStream(taskName, sn)
	}

	err = run.Start(ctx)
	duration := time.Since(start).Milliseconds()

	mw.CloseAll()

	// Finish streams with per-task status from the runner.
	for _, s := range streams {
		ts := run.TaskStatus(s.taskID)
		streamStatus := "skipped"
		switch ts {
		case runner.TaskStatusDone:
			streamStatus = "success"
		case runner.TaskStatusFailed:
			streamStatus = "failed"
		case runner.TaskStatusRunning:
			streamStatus = "success" // was still running when run ended normally
		}
		reporter.FinishStream(taskName, s.streamName, FinishStreamResult{
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
	sw := m.reporter.StreamWriter(m.jobName, encodeStreamName(id))
	m.writers[id] = sw
	return sw
}

func (m *streamMultiWriter) CloseAll() {
	for _, sw := range m.writers {
		sw.Close()
	}
}
