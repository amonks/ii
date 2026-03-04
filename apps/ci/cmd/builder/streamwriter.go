package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	flushInterval = 500 * time.Millisecond
	flushSize     = 8 * 1024 // 8KB
)

var streamRetry = retryConfig{
	maxAttempts: 5,
	baseDelay:   200 * time.Millisecond,
	maxDelay:    5 * time.Second,
}

// StreamWriter implements io.Writer by buffering writes and periodically
// flushing them to the orchestrator's output endpoint.
type StreamWriter struct {
	client  *http.Client
	url     string // full URL: {baseURL}/api/runs/{runID}/jobs/{name}/output/{stream}
	mu      sync.Mutex
	buf     bytes.Buffer
	done    chan struct{}
	closed  bool
	flushWg sync.WaitGroup
}

// NewStreamWriter creates a writer that streams output to the orchestrator.
func NewStreamWriter(client *http.Client, baseURL string, runID int64, jobName, stream string) *StreamWriter {
	sw := &StreamWriter{
		client: client,
		url:    fmt.Sprintf("%s/api/runs/%d/jobs/%s/output/%s", baseURL, runID, jobName, stream),
		done:   make(chan struct{}),
	}
	sw.flushWg.Go(sw.flushLoop)
	return sw
}

// Write buffers data and triggers a flush if the buffer exceeds the size threshold.
func (sw *StreamWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.closed {
		return 0, fmt.Errorf("stream writer closed")
	}
	n, err := sw.buf.Write(p)
	if sw.buf.Len() >= flushSize {
		sw.flushLocked()
	}
	return n, err
}

// Close flushes any remaining data and stops the flush loop.
func (sw *StreamWriter) Close() error {
	sw.mu.Lock()
	if sw.closed {
		sw.mu.Unlock()
		return nil
	}
	sw.closed = true
	sw.flushLocked()
	sw.mu.Unlock()

	close(sw.done)
	sw.flushWg.Wait()
	return nil
}

func (sw *StreamWriter) flushLoop() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-sw.done:
			return
		case <-ticker.C:
			sw.mu.Lock()
			sw.flushLocked()
			sw.mu.Unlock()
		}
	}
}

// flushLocked sends buffered data to the orchestrator. Must be called with mu held.
func (sw *StreamWriter) flushLocked() {
	if sw.buf.Len() == 0 {
		return
	}
	data := make([]byte, sw.buf.Len())
	copy(data, sw.buf.Bytes())
	sw.buf.Reset()

	// Send in background to avoid blocking writes.
	sw.flushWg.Go(func() {
		sw.send(data)
	})
}

func (sw *StreamWriter) send(data []byte) {
	resp, err := retryDo(sw.client, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, sw.url, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		return req, nil
	}, streamRetry)
	if err != nil {
		slog.Warn("stream send failed after retries, dropping data", "url", sw.url, "bytes", len(data), "error", err)
		return
	}
	resp.Body.Close()
}
