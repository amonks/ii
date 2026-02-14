package trafficclient

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"monks.co/pkg/middleware"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/traffic"
)

var appKey = &struct{}{}

// SetApp stores the matched app/route name for the current request's traffic log entry.
func SetApp(req *http.Request, name string) {
	if p, ok := req.Context().Value(appKey).(*string); ok {
		*p = name
	}
}

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
		app := new(string)
		req = req.WithContext(context.WithValue(req.Context(), appKey, app))

		start := time.Now()
		handler.ServeHTTP(w, req)
		dur := time.Since(start)

		c.enqueue(traffic.LogEntry{
			Timestamp:  start,
			Host:       req.Host,
			Path:       req.URL.Path,
			Query:      req.URL.RawQuery,
			Method:     req.Method,
			RemoteAddr: getRemoteAddr(req),
			UserAgent:  req.UserAgent(),
			Referer:    req.Header.Get("Referer"),
			StatusCode: reqlog.Status(req.Context()),
			Duration:   dur,
			App:        *app,
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

func getRemoteAddr(req *http.Request) string {
	if v, ok := req.Context().Value(reqlog.RemoteAddrKey).(string); ok {
		return v
	}
	return req.RemoteAddr
}
