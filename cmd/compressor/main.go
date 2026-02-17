package main

import (
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"github.com/andybalholm/brotli"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type compression struct {
	name      string
	ext       string
	compress  func(w io.Writer) io.WriteCloser
	skipEmpty bool
}

var compressions = []compression{
	{
		name: "gzip",
		ext:  ".gz",
		compress: func(w io.Writer) io.WriteCloser {
			gw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
			if err != nil {
				// This should never happen since we're using a predefined level
				panic(err)
			}
			return gw
		},
		skipEmpty: true, // gzip adds overhead for empty files
	},
	{
		name: "brotli",
		ext:  ".br",
		compress: func(w io.Writer) io.WriteCloser {
			return brotli.NewWriterLevel(w, brotli.BestCompression)
		},
		skipEmpty: true,
	},
}

type stats struct {
	filesProcessed atomic.Int64
	bytesProcessed atomic.Int64
	filesSkipped   atomic.Int64
	errors         atomic.Int64
	start          time.Time
}

func (s *stats) print() {
	elapsed := time.Since(s.start).Seconds()
	fmt.Printf("\nCompression completed in %.1f seconds:\n", elapsed)
	fmt.Printf("Files processed: %d\n", s.filesProcessed.Load())
	fmt.Printf("Bytes processed: %d\n", s.bytesProcessed.Load())
	fmt.Printf("Files skipped: %d\n", s.filesSkipped.Load())
	fmt.Printf("Errors: %d\n", s.errors.Load())

	if elapsed > 0 {
		mbPerSec := float64(s.bytesProcessed.Load()) / (1024 * 1024) / elapsed
		fmt.Printf("Average throughput: %.1f MB/s\n", mbPerSec)
	}
}

func main() {
	var dir string
	var workers int
	var verbose bool
	var force bool
	flag.StringVar(&dir, "dir", ".", "directory to walk")
	flag.IntVar(&workers, "workers", 4, "number of compression workers")
	flag.BoolVar(&verbose, "v", false, "verbose logging")
	flag.BoolVar(&force, "force", false, "force re-compression of all files")
	flag.Parse()

	logger := log.New(os.Stdout, "", log.Ltime)
	if !verbose {
		logger.SetOutput(io.Discard)
	}

	// Setup context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		logger.Println("Received interrupt signal, shutting down gracefully...")
		cancel()
	}()

	if err := walk(ctx, dir, workers, force, logger); err != nil {
		log.Fatal(err)
	}
}

func walk(ctx context.Context, root string, workers int, force bool, logger *log.Logger) error {
	stats := &stats{start: time.Now()}

	logger.Printf("Starting compression walk in %s with %d workers (force=%v)", root, workers, force)

	// Channel for sending work to compression workers
	jobs := make(chan string, workers*2)
	var wg sync.WaitGroup

	// Start worker pool
	for i := range workers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for path := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					if err := compressFile(path, stats, force, logger); err != nil {
						logger.Printf("Worker %d: Error compressing %s: %v", workerID, path, err)
						stats.errors.Add(1)
					}
				}
			}
		}(i)
	}

	// Walk the directory
	logger.Printf("Scanning directory %s", root)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Skip already compressed files
		if isCompressedFile(path) {
			stats.filesSkipped.Add(1)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case jobs <- path:
			return nil
		}
	})

	close(jobs)
	wg.Wait()
	stats.print()
	return err
}

func isCompressedFile(path string) bool {
	ext := filepath.Ext(path)
	for _, c := range compressions {
		if ext == c.ext {
			return true
		}
	}
	return false
}

func compressFile(path string, stats *stats, force bool, logger *log.Logger) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	// Skip empty files if compression type requires it
	if info.Size() == 0 {
		stats.filesSkipped.Add(1)
		return nil
	}

	logger.Printf("Processing %s (%d bytes)", path, info.Size())
	start := time.Now()

	for _, c := range compressions {
		compressedPath := path + c.ext

		// Check if compressed file exists and is newer, unless force is true
		if !force {
			if compressedInfo, err := os.Stat(compressedPath); err == nil {
				if compressedInfo.ModTime().After(info.ModTime()) {
					logger.Printf("Skipping %s compression for %s (already up to date)", c.name, path)
					continue
				}
			}
		}

		// Skip empty files if this compression type requires it
		if c.skipEmpty && info.Size() == 0 {
			continue
		}

		if err := compressOneFile(path, compressedPath, c.compress); err != nil {
			return fmt.Errorf("%s compression of %s: %w", c.name, path, err)
		}

		// Set the compressed file's modification time to match the original
		if err := os.Chtimes(compressedPath, time.Now(), info.ModTime()); err != nil {
			logger.Printf("Warning: couldn't set modification time for %s: %v", compressedPath, err)
		}

		compressedInfo, err := os.Stat(compressedPath)
		if err == nil {
			logger.Printf("Compressed %s to %s (%d -> %d bytes, %.1f%%)",
				path, compressedPath, info.Size(), compressedInfo.Size(),
				float64(compressedInfo.Size())/float64(info.Size())*100)
		}
	}

	stats.filesProcessed.Add(1)
	stats.bytesProcessed.Add(info.Size())
	logger.Printf("Completed %s in %v", path, time.Since(start))
	return nil
}

func compressOneFile(src, dst string, newCompressor func(io.Writer) io.WriteCloser) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer out.Close()

	compressor := newCompressor(out)
	if _, err := io.Copy(compressor, in); err != nil {
		compressor.Close()
		return fmt.Errorf("compress: %w", err)
	}
	if err := compressor.Close(); err != nil {
		return fmt.Errorf("finalize: %w", err)
	}
	return nil
}
