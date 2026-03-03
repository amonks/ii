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

func TestCompressFileSkipsUpToDate(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("Hello, World!"), 0644); err != nil {
		t.Fatal(err)
	}

	logger := log.New(io.Discard, "", 0)

	// First compression: should create .gz and .br files.
	stats1 := &stats{start: time.Now()}
	if err := compressFile(path, stats1, false, logger); err != nil {
		t.Fatal(err)
	}
	if stats1.filesProcessed.Load() != 1 {
		t.Fatalf("expected 1 file processed on first run, got %d", stats1.filesProcessed.Load())
	}

	// Record mtime of compressed files.
	gzBefore, err := os.Stat(path + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	brBefore, err := os.Stat(path + ".br")
	if err != nil {
		t.Fatal(err)
	}

	// Second compression without force: should skip because compressed
	// files are up to date.
	stats2 := &stats{start: time.Now()}
	if err := compressFile(path, stats2, false, logger); err != nil {
		t.Fatal(err)
	}

	// The compressed files should not have been rewritten.
	gzAfter, err := os.Stat(path + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	brAfter, err := os.Stat(path + ".br")
	if err != nil {
		t.Fatal(err)
	}
	if !gzAfter.ModTime().Equal(gzBefore.ModTime()) {
		t.Errorf(".gz file was rewritten (mtime changed from %v to %v)", gzBefore.ModTime(), gzAfter.ModTime())
	}
	if !brAfter.ModTime().Equal(brBefore.ModTime()) {
		t.Errorf(".br file was rewritten (mtime changed from %v to %v)", brBefore.ModTime(), brAfter.ModTime())
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

	// Test cancellation deterministically: pre-cancel the context and use
	// enough files to overflow the jobs channel buffer (workers*2). With
	// the context already canceled, workers exit immediately without
	// draining the channel, so the walk callback eventually blocks on
	// the full channel and must take the ctx.Done() branch.
	cancelDir := t.TempDir()
	for i := range 20 {
		p := filepath.Join(cancelDir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err = walk(cancelCtx, cancelDir, 1, false, logger)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
