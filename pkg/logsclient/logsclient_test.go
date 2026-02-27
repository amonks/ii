package logsclient

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// collectServer returns an httptest.Server that collects all posted
// batches and a function to retrieve them.
func collectServer(t *testing.T) (*httptest.Server, func() [][]json.RawMessage) {
	t.Helper()
	var (
		mu      sync.Mutex
		batches [][]json.RawMessage
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			http.Error(w, "bad", 500)
			return
		}
		var batch []json.RawMessage
		if err := json.Unmarshal(body, &batch); err != nil {
			t.Errorf("unmarshal: %v", err)
			http.Error(w, "bad", 500)
			return
		}
		mu.Lock()
		batches = append(batches, batch)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	return srv, func() [][]json.RawMessage {
		mu.Lock()
		defer mu.Unlock()
		cp := make([][]json.RawMessage, len(batches))
		copy(cp, batches)
		return cp
	}
}

func writeLine(t *testing.T, c *Client, msg string) {
	t.Helper()
	line, err := json.Marshal(map[string]string{"msg": msg})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(line); err != nil {
		t.Fatal(err)
	}
}

func TestNotFlushedBeforeReady(t *testing.T) {
	srv, getBatches := collectServer(t)
	ready := make(chan struct{}) // never closed

	c := New(srv.URL, srv.Client(), ready)
	defer c.Close()

	writeLine(t, c, "hello")

	// Give the flush loop time to potentially fire (it shouldn't).
	time.Sleep(100 * time.Millisecond)

	if batches := getBatches(); len(batches) != 0 {
		t.Errorf("expected 0 batches before ready, got %d", len(batches))
	}
}

func TestReadyTriggersFlushedBufferedLogs(t *testing.T) {
	srv, getBatches := collectServer(t)
	ready := make(chan struct{})

	c := New(srv.URL, srv.Client(), ready)
	defer c.Close()

	writeLine(t, c, "buffered-1")
	writeLine(t, c, "buffered-2")

	// Signal ready.
	close(ready)

	// Wait for the flush loop to pick up the ready signal and flush.
	deadline := time.After(2 * time.Second)
	for {
		if batches := getBatches(); len(batches) > 0 {
			total := 0
			for _, b := range batches {
				total += len(b)
			}
			if total >= 2 {
				break
			}
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for buffered logs to flush after ready")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestBufferTrimmedWhenNotReady(t *testing.T) {
	srv, _ := collectServer(t)
	ready := make(chan struct{}) // never closed

	c := New(srv.URL, srv.Client(), ready)
	defer c.Close()

	// Write more than batchSize entries.
	for i := range batchSize + 50 {
		writeLine(t, c, string(rune('A'+i%26)))
	}

	c.mu.Lock()
	n := len(c.buf)
	c.mu.Unlock()

	if n > batchSize {
		t.Errorf("buffer should be trimmed to %d, got %d", batchSize, n)
	}
}

func TestNormalFlushAfterReady(t *testing.T) {
	srv, getBatches := collectServer(t)
	ready := make(chan struct{})
	close(ready) // already ready

	c := New(srv.URL, srv.Client(), ready)
	defer c.Close()

	// Wait for the initial (empty) flush from flushLoop to pass.
	time.Sleep(50 * time.Millisecond)

	writeLine(t, c, "after-ready")

	// Close triggers a final flush.
	c.Close()

	batches := getBatches()
	total := 0
	for _, b := range batches {
		total += len(b)
	}
	if total < 1 {
		t.Errorf("expected at least 1 flushed event after ready, got %d", total)
	}
}
