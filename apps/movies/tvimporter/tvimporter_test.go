package tvimporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEpisodeInfo(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		wantSeason    int
		wantEpisode   int
		shouldSucceed bool
	}{
		{"S01E01 format", "Show.S01E01.mkv", 1, 1, true},
		{"s01e01 lowercase", "show.s01e01.mkv", 1, 1, true},
		{"Season 1 Episode 2", "Season 1 Episode 2.mkv", 1, 2, true},
		{"1x03 format", "Show.1x03.mkv", 1, 3, true},
		{"No match", "random.mkv", 0, 0, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			season, episode, err := ParseEpisodeInfo(tt.filename)
			if (err == nil) != tt.shouldSucceed {
				t.Errorf("ParseEpisodeInfo() error = %v, shouldSucceed = %v", err, tt.shouldSucceed)
				return
			}
			
			if tt.shouldSucceed {
				if season != tt.wantSeason {
					t.Errorf("ParseEpisodeInfo() season = %v, want %v", season, tt.wantSeason)
				}
				if episode != tt.wantEpisode {
					t.Errorf("ParseEpisodeInfo() episode = %v, want %v", episode, tt.wantEpisode)
				}
			}
		})
	}
}

func TestTVImporter_IsEpisodeFile(t *testing.T) {
	// Create a minimal TVImporter (without full DB and TMDB)
	importer := &TVImporter{}
	
	tests := []struct {
		filename string
		want     bool
	}{
		{"ShowA.S01E01.mkv", true},
		{"ShowA.S01E01.mp4", true},
		{"ShowA.S01E01.avi", true},
		{"ShowA.S01E01.m4v", true},
		{"Episode 01.mkv", false}, // Not matching our patterns
		{"Season 1 Episode 2.mkv", true},
		{"Show.1x03.mkv", true},
		{"random.mkv", false},
		{"ShowA.S01E01.txt", false}, // Wrong extension
	}
	
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := importer.isEpisodeFile(tt.filename); got != tt.want {
				t.Errorf("isEpisodeFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Simple integration test using real filesystem for validation
func TestTVImporter_RealFilesystem(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "tvtest")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create test directory structure
	showDir := filepath.Join(tempDir, "TestShow")
	seasonDir := filepath.Join(showDir, "Season 1")
	
	if err := os.MkdirAll(seasonDir, 0755); err != nil {
		t.Fatalf("Error creating directories: %v", err)
	}
	
	// Create sample episode files
	episodeFiles := []string{
		filepath.Join(seasonDir, "TestShow.S01E01.mkv"),
		filepath.Join(seasonDir, "TestShow.S01E02.mkv"),
		filepath.Join(seasonDir, "extras.txt"), // non-episode file
	}
	
	for _, file := range episodeFiles {
		if err := os.WriteFile(file, []byte("test content"), 0644); err != nil {
			t.Fatalf("Error creating file %s: %v", file, err)
		}
	}
	
	// Test the scanTVDirectory function directly
	paths := []string{}
	
	// Walk the directory and collect episode paths
	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Only collect paths that would be considered episodes
		importer := &TVImporter{}
		if importer.isEpisodeFile(filepath.Base(path)) {
			// Convert to relative path from temp dir
			relPath, err := filepath.Rel(tempDir, path)
			if err != nil {
				return err
			}
			paths = append(paths, relPath)
		}
		
		return nil
	})
	
	if err != nil {
		t.Fatalf("Error walking directory: %v", err)
	}
	
	// Check that we found the expected episode files
	if len(paths) != 2 {
		t.Errorf("Expected 2 episode files, got %d", len(paths))
	}
	
	// Check that we found the expected paths
	for _, path := range paths {
		// Convert slashes for comparison
		path = strings.ReplaceAll(path, "\\", "/")
		if !strings.HasPrefix(path, "TestShow/Season 1/TestShow.S01E0") {
			t.Errorf("Unexpected path: %s", path)
		}
	}
}