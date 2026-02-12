package trafficclient

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"monks.co/pkg/middleware"
	"monks.co/pkg/traffic"
)

var RemoteAddrKey = &struct{}{}

var _ middleware.Middleware = &Client{}

type Client struct {
	url    string
	client *http.Client

	mu   sync.Mutex
	buf  []traffic.LogEntry
	done chan struct{}
}

func New(trafficURL string, httpClient *http.Client) *Client {
	c := &Client{
		url:    trafficURL,
		client: httpClient,
		done:   make(chan struct{}),
	}
	go c.flushLoop()
	return c
}

func (c *Client) Close() {
	close(c.done)
	c.flush()
}

func (c *Client) ModifyHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ww := &statusRecorder{ResponseWriter: w}

		start := time.Now()
		handler.ServeHTTP(ww, req)
		dur := time.Since(start)

		c.enqueue(traffic.LogEntry{
			Timestamp:  start,
			Host:       req.Host,
			Path:       req.URL.Path,
			Query:      req.URL.RawQuery,
			RemoteAddr: getRemoteAddr(req),
			UserAgent:  req.UserAgent(),
			Referer:    req.Header.Get("Referer"),
			StatusCode: ww.status,
			Duration:   dur,
		})
	})
}

func (c *Client) enqueue(entry traffic.LogEntry) {
	c.mu.Lock()
	c.buf = append(c.buf, entry)
	full := len(c.buf) >= 100
	c.mu.Unlock()
	if full {
		go c.flush()
	}
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
	entries := c.buf
	c.buf = nil
	c.mu.Unlock()

	if len(entries) == 0 {
		return
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(entries); err != nil {
		log.Printf("trafficclient: encode error: %v", err)
		return
	}

	resp, err := c.client.Post(c.url, "application/json", &buf)
	if err != nil {
		log.Printf("trafficclient: post error: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("trafficclient: unexpected status %d", resp.StatusCode)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func getRemoteAddr(req *http.Request) string {
	if v, ok := req.Context().Value(RemoteAddrKey).(string); ok {
		return v
	}
	return req.RemoteAddr
}
