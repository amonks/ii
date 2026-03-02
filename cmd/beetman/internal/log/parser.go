package log

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Parser handles parsing of beet's log files
type Parser struct {
	albumsDir string // Base directory to strip from paths
}

// New creates a new log parser
func New(albumsDir string) *Parser {
	return &Parser{
		albumsDir: filepath.Clean(albumsDir),
	}
}

// ParseSkippedAlbums parses a log file to identify which albums from a batch
// were skipped, and why. Returns a map from album path to skip reason.
func (p *Parser) ParseSkippedAlbums(logFile string) (map[string]string, error) {
	file, err := os.Open(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No log file means no skips
		}
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	skipped := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		var reason, path string
		if strings.HasPrefix(line, "duplicate-skip ") {
			reason = "duplicate"
			path = strings.TrimPrefix(line, "duplicate-skip ")
		} else if strings.HasPrefix(line, "skip ") {
			reason = "no strong match"
			path = strings.TrimPrefix(line, "skip ")
		} else {
			continue
		}

		// Strip multi-disc paths after semicolons
		if idx := strings.Index(path, ";"); idx >= 0 {
			path = path[:idx]
		}
		path = strings.TrimSpace(path)
		path = filepath.Clean(path)

		// Convert to relative path
		parts := strings.Split(path, "/files/flac/")
		if len(parts) != 2 {
			panic(fmt.Errorf("unexpected path: %s", path))
		}
		path = parts[1]

		skipped[path] = reason
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return skipped, nil
}
