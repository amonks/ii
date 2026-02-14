package logsclient

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// Client implements io.Writer. Each Write receives one complete JSON line
// from slog.JSONHandler. Events are buffered and flushed in batches.
type Client struct {
	url  string
	mu   sync.Mutex
	buf  []json.RawMessage
	done chan struct{}
}

// New creates a new log shipping client that sends batched events to url.
func New(url string) *Client {
	c := &Client{
		url:  url,
		done: make(chan struct{}),
	}
	go c.flushLoop()
	return c
}

// Write buffers a single JSON log line. It always returns len(p), nil
// so it never blocks slog.
func (c *Client) Write(p []byte) (int, error) {
	// Make a copy since p may be reused by the caller.
	cp := make([]byte, len(p))
	copy(cp, p)

	c.mu.Lock()
	c.buf = append(c.buf, json.RawMessage(cp))
	full := len(c.buf) >= 100
	c.mu.Unlock()

	if full {
		go c.flush()
	}
	return len(p), nil
}

// Close flushes remaining events and stops the flush goroutine.
func (c *Client) Close() error {
	close(c.done)
	c.flush()
	return nil
}

func (c *Client) flushLoop() {
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

	resp, err := http.Post(c.url, "application/json", &buf)
	if err != nil {
		log.Printf("logsclient: post error: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("logsclient: unexpected status %d", resp.StatusCode)
	}
}
