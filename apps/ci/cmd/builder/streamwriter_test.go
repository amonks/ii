package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestStreamWriterFlushesOnClose(t *testing.T) {
	var mu sync.Mutex
	var received []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/runs/1/jobs/test/output/default" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		data, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, data...)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sw := NewStreamWriter(http.DefaultClient, srv.URL, 1, "test", "default")
	sw.Write([]byte("hello world"))
	sw.Close()

	mu.Lock()
	defer mu.Unlock()
	if string(received) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(received))
	}
}

func TestStreamWriterFlushesOnInterval(t *testing.T) {
	var mu sync.Mutex
	var received []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, data...)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sw := NewStreamWriter(http.DefaultClient, srv.URL, 1, "test", "out")
	sw.Write([]byte("tick"))

	// Wait for a flush interval + some margin.
	time.Sleep(flushInterval + 100*time.Millisecond)

	mu.Lock()
	got := string(received)
	mu.Unlock()

	if got != "tick" {
		t.Errorf("expected 'tick' after interval, got %q", got)
	}

	sw.Close()
}

func TestStreamWriterFlushesOnSizeThreshold(t *testing.T) {
	var mu sync.Mutex
	var received []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, data...)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sw := NewStreamWriter(http.DefaultClient, srv.URL, 1, "test", "out")

	// Write more than flushSize bytes.
	big := make([]byte, flushSize+1)
	for i := range big {
		big[i] = 'x'
	}
	sw.Write(big)
	sw.Close()

	mu.Lock()
	got := len(received)
	mu.Unlock()

	if got != flushSize+1 {
		t.Errorf("expected %d bytes flushed, got %d", flushSize+1, got)
	}
}

func TestStreamWriterRetriesOnServerError(t *testing.T) {
	var mu sync.Mutex
	var attempts int
	var received []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		mu.Lock()
		attempts++
		a := attempts
		mu.Unlock()
		if a == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		mu.Lock()
		received = append(received, data...)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sw := NewStreamWriter(http.DefaultClient, srv.URL, 1, "test", "default")
	sw.Write([]byte("retry-data"))
	sw.Close()

	mu.Lock()
	defer mu.Unlock()
	if string(received) != "retry-data" {
		t.Errorf("expected 'retry-data', got %q", string(received))
	}
	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
}

func TestStreamWriterDropsAfterRetryExhaustion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	sw := NewStreamWriter(http.DefaultClient, srv.URL, 1, "test", "default")
	sw.Write([]byte("will-be-dropped"))

	// Close should complete without hanging or panicking.
	sw.Close()
}

func TestReporterStreamWriter(t *testing.T) {
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	reporter := NewReporter(srv.URL, 42, http.DefaultClient)
	sw := reporter.StreamWriter("deploy-dogs", "output")
	sw.Write([]byte("test"))
	sw.Close()

	if gotPath != "/api/runs/42/jobs/deploy-dogs/output/output" {
		t.Errorf("unexpected path: %s", gotPath)
	}
}
