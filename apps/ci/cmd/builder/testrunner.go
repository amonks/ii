package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"monks.co/run/runner"
	"monks.co/run/taskfile"
)

type streamMapping struct {
	taskID     string
	streamName string
}

// encodeStreamName makes a task ID safe for use as a single URL path segment
// by replacing "/" with "~".
func encodeStreamName(name string) string {
	return strings.ReplaceAll(name, "/", "~")
}

// RunTests runs the generate and test tasks using the run library
// programmatically, streaming per-task output to the orchestrator.
// The suffix is appended to job names (e.g. "-2" gives "generate-2", "ci-test-2").
func RunTests(ctx context.Context, root string, reporter *Reporter, suffix string) error {
	if err := runTask(ctx, root, "generate", "generate"+suffix, reporter); err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if err := runTask(ctx, root, "ci-test", "ci-test"+suffix, reporter); err != nil {
		return fmt.Errorf("ci-test: %w", err)
	}
	return nil
}

func runTask(ctx context.Context, root, taskName, jobName string, reporter *Reporter) error {
	reporter.StartJob(jobName, "task")
	start := time.Now()

	tasks, err := taskfile.Load(root)
	if err != nil {
		errMsg := fmt.Sprintf("loading taskfile: %v", err)
		reporter.FinishJob(jobName, FinishJobResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("loading taskfile: %w", err)
	}

	mw := &streamMultiWriter{
		reporter:  reporter,
		jobName:   jobName,
		writers:   make(map[string]*StreamWriter),
		finished:  make(map[string]bool),
		startTime: make(map[string]time.Time),
	}

	run, err := runner.New(runner.RunTypeShort, root, tasks, taskName, mw)
	if err != nil {
		errMsg := fmt.Sprintf("creating runner: %v", err)
		reporter.FinishJob(jobName, FinishJobResult{
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
	for _, id := range ids {
		if strings.HasPrefix(id, "@") {
			continue
		}
		sn := encodeStreamName(id)
		mw.streams = append(mw.streams, streamMapping{taskID: id, streamName: sn})
		reporter.StartStream(jobName, sn)
	}
	mw.run = run

	err = run.Start(ctx)
	duration := time.Since(start).Milliseconds()

	mw.CloseAll()

	// Report any streams that weren't already reported during execution.
	mw.checkStatuses()
	// Report any remaining streams (e.g. tasks that were skipped or never ran).
	mw.finishRemaining()

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

	if err != nil {
		return fmt.Errorf("%s: %w", taskName, err)
	}
	return nil
}

// streamMultiWriter implements runner.MultiWriter, returning a StreamWriter
// per task ID so each task's output is streamed separately. On each write,
// it checks all tasks' statuses and reports any newly-finished streams to
// the orchestrator, so the web UI updates incrementally.
type streamMultiWriter struct {
	reporter  *Reporter
	jobName   string
	writers   map[string]*StreamWriter
	run       *runner.Run
	streams   []streamMapping
	mu        sync.Mutex
	finished  map[string]bool
	startTime map[string]time.Time // first write time per task ID
}

func (m *streamMultiWriter) Writer(id string) io.Writer {
	sw := m.reporter.StreamWriter(m.jobName, encodeStreamName(id))
	m.writers[id] = sw
	return &statusCheckingWriter{inner: sw, mw: m, taskID: id}
}

func (m *streamMultiWriter) CloseAll() {
	for _, sw := range m.writers {
		sw.Close()
	}
}

// checkStatuses reports any streams whose tasks have finished but haven't
// been reported yet.
func (m *streamMultiWriter) checkStatuses() {
	if m.run == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, s := range m.streams {
		if m.finished[s.taskID] {
			continue
		}
		ts := m.run.TaskStatus(s.taskID)
		var status string
		switch ts {
		case runner.TaskStatusDone:
			status = "success"
		case runner.TaskStatusFailed:
			status = "failed"
		default:
			continue
		}
		m.finished[s.taskID] = true
		var durationMs int64
		if t, ok := m.startTime[s.taskID]; ok {
			durationMs = now.Sub(t).Milliseconds()
		}
		m.reporter.FinishStream(m.jobName, s.streamName, FinishStreamResult{
			Status:     status,
			DurationMs: durationMs,
		})
	}
}

// finishRemaining reports any streams not yet reported as "skipped".
func (m *streamMultiWriter) finishRemaining() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.streams {
		if m.finished[s.taskID] {
			continue
		}
		m.finished[s.taskID] = true
		m.reporter.FinishStream(m.jobName, s.streamName, FinishStreamResult{
			Status: "skipped",
		})
	}
}

// statusCheckingWriter wraps a StreamWriter and checks all task statuses
// after each write, so finished tasks are reported incrementally.
type statusCheckingWriter struct {
	inner  *StreamWriter
	mw     *streamMultiWriter
	taskID string
}

func (w *statusCheckingWriter) Write(p []byte) (int, error) {
	w.mw.mu.Lock()
	if _, ok := w.mw.startTime[w.taskID]; !ok {
		w.mw.startTime[w.taskID] = time.Now()
	}
	w.mw.mu.Unlock()

	n, err := w.inner.Write(p)
	w.mw.checkStatuses()
	return n, err
}
