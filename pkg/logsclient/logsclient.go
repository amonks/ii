package logsclient

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

const batchSize = 100

// Client implements io.Writer. Each Write receives one complete JSON line
// from slog.JSONHandler. Events are buffered and flushed in batches.
type Client struct {
	url      string
	client   *http.Client
	ready    <-chan struct{}
	mu       sync.Mutex
	buf      []json.RawMessage
	done     chan struct{}
	closeOnce sync.Once
}

// New creates a new log shipping client that sends batched events to url
// using the provided HTTP client. Flushes are gated on ready: no network
// calls are made until the ready channel is closed.
func New(url string, client *http.Client, ready <-chan struct{}) *Client {
	c := &Client{
		url:    url,
		client: client,
		ready:  ready,
		done:   make(chan struct{}),
	}
	go c.flushLoop()
	return c
}

// Write buffers a single JSON log line. It always returns len(p), nil
// so it never blocks slog. When not ready and the buffer exceeds batch
// size, the oldest entries are dropped.
func (c *Client) Write(p []byte) (int, error) {
	// Make a copy since p may be reused by the caller.
	cp := make([]byte, len(p))
	copy(cp, p)

	c.mu.Lock()
	c.buf = append(c.buf, json.RawMessage(cp))
	if !c.isReady() && len(c.buf) > batchSize {
		c.buf = c.buf[len(c.buf)-batchSize:]
	}
	full := c.isReady() && len(c.buf) >= batchSize
	c.mu.Unlock()

	if full {
		go c.flush()
	}
	return len(p), nil
}

// Close flushes remaining events and stops the flush goroutine.
// It is safe to call multiple times.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.isReady() {
			c.flush()
		}
	})
	return nil
}

// isReady reports whether the ready channel has been closed.
// Must not be called without the caller understanding it may race;
// used inside mu-locked sections or for best-effort checks.
func (c *Client) isReady() bool {
	select {
	case <-c.ready:
		return true
	default:
		return false
	}
}

func (c *Client) flushLoop() {
	// Wait for readiness before entering the tick loop.
	select {
	case <-c.ready:
	case <-c.done:
		return
	}

	// Immediate flush of anything buffered during startup.
	c.flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.flush()
		case <-c.done:
			return
		}
	}
}

func (c *Client) flush() {
	c.mu.Lock()
	events := c.buf
	c.buf = nil
	c.mu.Unlock()

	if len(events) == 0 {
		return
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(events); err != nil {
		log.Printf("logsclient: encode error: %v", err)
		return
	}

	resp, err := c.client.Post(c.url, "application/json", &buf)
	if err != nil {
		log.Printf("logsclient: post error: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("logsclient: unexpected status %d", resp.StatusCode)
	}
}
