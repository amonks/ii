package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsCompressedFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"file.txt", false},
		{"file.txt.gz", true},
		{"file.txt.br", true},
		{"file.gz.txt", false},
		{"file.br", true},
		{"", false},
		{".gz", true},
		{".br", true},
	}

	for _, tt := range tests {
		if got := isCompressedFile(tt.path); got != tt.want {
			t.Errorf("isCompressedFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestCompressFile(t *testing.T) {
	// Create a temporary directory for testing
	dir, err := os.MkdirTemp("", "compress_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create test files with different sizes
	testFiles := []struct {
		name    string
		content string
	}{
		{"empty.txt", ""},
		{"small.txt", "Hello, World!"},
		{"medium.txt", string(make([]byte, 1024))},
	}

	for _, tf := range testFiles {
		path := filepath.Join(dir, tf.name)
		if err := os.WriteFile(path, []byte(tf.content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	stats := &stats{start: time.Now()}
	logger := log.New(io.Discard, "", 0)

	// Test compression
	for _, tf := range testFiles {
		path := filepath.Join(dir, tf.name)
		if err := compressFile(path, stats, false, logger); err != nil {
			t.Errorf("compressFile(%q) error: %v", path, err)
		}
	}

	// Test force compression
	path := filepath.Join(dir, "small.txt")
	if err := compressFile(path, stats, true, logger); err != nil {
		t.Errorf("force compressFile(%q) error: %v", path, err)
	}
}

func TestWalk(t *testing.T) {
	dir, err := os.MkdirTemp("", "walk_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create test directory structure
	files := []struct {
		path    string
		content string
	}{
		{"file1.txt", "content1"},
		{"subdir/file2.txt", "content2"},
		{"subdir/file3.txt.gz", "already compressed"},
		{"empty.txt", ""},
	}

	for _, f := range files {
		path := filepath.Join(dir, f.path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	logger := log.New(io.Discard, "", 0)

	// Test with different worker counts
	for _, workers := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			if err := walk(ctx, dir, workers, false, logger); err != nil {
				t.Errorf("walk error with %d workers: %v", workers, err)
			}
		})
	}

	// Test cancellation
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err = walk(ctx, dir, 1, false, logger)
	if err != ctx.Err() {
		t.Errorf("expected context cancellation error, got %v", err)
	}
}


