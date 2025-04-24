package tvimporter

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
	"monks.co/pkg/filesystem"
	"monks.co/pkg/tmdb"
)

var (
	// Common TV show episode patterns
	episodePattern      = regexp.MustCompile(`(?i)S(\d+)E(\d+)`)
	seasonEpPattern     = regexp.MustCompile(`(?i)Season\s*(\d+).*?Episode\s*(\d+)`)
	dotSeasonEpPattern  = regexp.MustCompile(`(\d+)x(\d+)`)
	seasonFolderPattern = regexp.MustCompile(`(?i)Season\s*(\d+)`)
)

type TVImporter struct {
	tmdb *tmdb.Client
	db   *db.DB
	fs   filesystem.FS
}

func New(tmdb *tmdb.Client, db *db.DB) *TVImporter {
	return &TVImporter{
		tmdb: tmdb,
		db:   db,
		fs:   filesystem.NewOSFileSystem(),
	}
}

// WithFS allows injecting a custom filesystem for testing
func (app *TVImporter) WithFS(fs filesystem.FS) *TVImporter {
	app.fs = fs
	return app
}

func (app *TVImporter) Run(ctx context.Context) error {
	log.Println("tvimporter started")
	defer log.Println("tvimporter done")

	// Count existing stubs before running the import
	existingStubs, err := app.db.CountStubsByType(db.MediaTypeTV)
	if err != nil {
		log.Printf("Error counting existing TV stubs: %v", err)
	}

	// Process the TV import directory
	if err := app.scanTVDirectory(ctx, config.TVImportDir); err != nil {
		return fmt.Errorf("error scanning TV directory: %w", err)
	}

	// Count stubs after running the import
	newStubsCount, err := app.db.CountStubsByType(db.MediaTypeTV)
	if err != nil {
		log.Printf("Error counting TV stubs: %v", err)
	} else {
		stubsAdded := newStubsCount - existingStubs
		log.Printf("TV import scan complete. Added %d new TV show stubs.", stubsAdded)
	}

	return nil
}

func (app *TVImporter) scanTVDirectory(ctx context.Context, rootDir string) error {
	// First, identify potential TV show directories
	entries, err := app.fs.ReadDir(rootDir)
	if err != nil {
		return fmt.Errorf("error reading TV directory: %w", err)
	}

	// Log summary instead of individual entries
	directoryCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			directoryCount++
		}
	}
	log.Printf("Found %d potential TV show directories to scan", directoryCount)

	// For each possible show directory
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		if !entry.IsDir() {
			continue // Skip files at root level - we're looking for show folders
		}

		showDir := app.fs.Join(rootDir, entry.Name())
		showRelPath := entry.Name()

		// Create a stub for the show directory itself
		if err := app.processShowDirectory(ctx, showDir, showRelPath); err != nil {
			log.Printf("Error processing show directory %s: %v", showDir, err)
			continue
		}
	}

	return nil
}

func (app *TVImporter) processShowDirectory(ctx context.Context, showDir, showRelPath string) error {
	// Check if this show is already in the database or ignored
	if ignored, err := app.db.PathIsIgnored(db.MediaTypeTV, showRelPath); err != nil {
		return fmt.Errorf("error checking if ignore exists for show '%s': %w", showRelPath, err)
	} else if ignored {
		return nil
	}

	// Find all episode files in the directory
	episodeFiles, err := app.findEpisodeFiles(ctx, showDir, showRelPath)
	if err != nil {
		return fmt.Errorf("error finding episode files in show '%s': %w", showRelPath, err)
	}

	// Check if any episode files have already been imported
	for _, episodePath := range episodeFiles {
		fullPath := filepath.Join(config.TVImportDir, episodePath)
		if exists, err := app.db.TVShowExistsFromPath(fullPath); err != nil {
			log.Printf("Warning: Error checking if TV episode exists from path '%s': %v", episodePath, err)
		} else if exists {
			return nil
		}
	}

	// Check if a stub already exists for this show directory
	stubExists := false
	var existingStub *db.Stub
	if exists, err := app.db.StubExistsFromPath(db.MediaTypeTV, showRelPath); err != nil {
		return fmt.Errorf("error checking if stub exists for show '%s': %w", showRelPath, err)
	} else {
		stubExists = exists
		if exists {
			existingStub, err = app.db.GetStub(showRelPath)
			if err != nil {
				return fmt.Errorf("error getting existing stub for show '%s': %w", showRelPath, err)
			}
		}
	}

	if len(episodeFiles) == 0 {
		// Skip directories that don't contain any valid TV episode files
		log.Printf("Skipping directory with no valid episodes: %s", showRelPath)
		return nil
	}

	if stubExists {
		// Only update if episode list has changed
		episodesChanged := !episodeFilesEqual(existingStub.EpisodeFiles, episodeFiles)

		if episodesChanged {
			// Update existing stub with episode files
			existingStub.EpisodeFiles = episodeFiles
			if err := app.db.SaveStub(existingStub); err != nil {
				return fmt.Errorf("error updating stub with episode files for show '%s': %w", showRelPath, err)
			}
			log.Printf("Updated TV show stub with %d episode files: %s", len(episodeFiles), showRelPath)
		}
	} else {
		// Create a new stub for this show directory
		stub, err := app.db.CreateStub(db.MediaTypeTV, showRelPath)
		if err != nil {
			return fmt.Errorf("error creating stub for show '%s': %w", showRelPath, err)
		}

		// Update the stub with episode files
		stub.EpisodeFiles = episodeFiles
		if err := app.db.SaveStub(stub); err != nil {
			return fmt.Errorf("error saving episode files to stub for show '%s': %w", showRelPath, err)
		}

		log.Printf("Added TV show stub with %d episode files: %s", len(episodeFiles), showRelPath)
	}

	return nil
}

// findEpisodeFiles recursively searches a directory and returns all valid TV episode files
func (app *TVImporter) findEpisodeFiles(ctx context.Context, dir, basePath string) ([]string, error) {
	var episodeFiles []string

	var searchDir func(string, string) error
	searchDir = func(curDir, relPath string) error {
		entries, err := app.fs.ReadDir(curDir)
		if err != nil {
			return fmt.Errorf("error reading directory: %w", err)
		}

		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}

			path := app.fs.Join(curDir, entry.Name())
			entryRelPath := app.fs.Join(relPath, entry.Name())

			if entry.IsDir() {
				// Recursively search subdirectories
				if err := searchDir(path, entryRelPath); err != nil {
					log.Printf("Error searching subdirectory %s: %v", path, err)
				}
			} else if app.isEpisodeFile(entry.Name()) {
				// Add episode file to the list
				episodeFiles = append(episodeFiles, entryRelPath)
			}
		}

		return nil
	}

	if err := searchDir(dir, basePath); err != nil {
		return nil, err
	}

	return episodeFiles, nil
}

func (app *TVImporter) isEpisodeFile(filename string) bool {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".mkv" && ext != ".mp4" && ext != ".avi" && ext != ".m4v" {
		return false
	}

	// Check for episode patterns in filename
	// Note: we're using case-insensitive regex patterns ((?i) prefix), so we don't need to lowercase the filename
	return episodePattern.MatchString(filename) ||
		seasonEpPattern.MatchString(filename) ||
		dotSeasonEpPattern.MatchString(filename)
}

// ParseEpisodeInfo extracts season and episode numbers from a filename
func ParseEpisodeInfo(filename string) (int, int, error) {
	// Try various patterns to extract season and episode numbers
	if match := episodePattern.FindStringSubmatch(filename); match != nil {
		seasonNum := match[1]
		episodeNum := match[2]
		season, _ := strconv.Atoi(seasonNum)
		episode, _ := strconv.Atoi(episodeNum)
		return season, episode, nil
	}

	if match := seasonEpPattern.FindStringSubmatch(filename); match != nil {
		seasonNum := match[1]
		episodeNum := match[2]
		season, _ := strconv.Atoi(seasonNum)
		episode, _ := strconv.Atoi(episodeNum)
		return season, episode, nil
	}

	if match := dotSeasonEpPattern.FindStringSubmatch(filename); match != nil {
		seasonNum := match[1]
		episodeNum := match[2]
		season, _ := strconv.Atoi(seasonNum)
		episode, _ := strconv.Atoi(episodeNum)
		return season, episode, nil
	}

	// Try looking for "Season X" directory in filepath and number in filename
	parts := strings.Split(filepath.Dir(filename), string(filepath.Separator))
	for _, part := range parts {
		if seasonMatch := seasonFolderPattern.FindStringSubmatch(part); seasonMatch != nil {
			season, _ := strconv.Atoi(seasonMatch[1])

			// Now try to find an episode number in the filename
			episodeMatch := regexp.MustCompile(`(\d+)`).FindStringSubmatch(filepath.Base(filename))
			if episodeMatch != nil {
				episode, _ := strconv.Atoi(episodeMatch[1])
				return season, episode, nil
			}
		}
	}

	return 0, 0, fmt.Errorf("could not parse season and episode from filename: %s", filename)
}

// episodeFilesEqual compares two slices of episode files to determine if they contain the same elements
func episodeFilesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps to count occurrences of each path
	countA := make(map[string]int)
	countB := make(map[string]int)

	for _, path := range a {
		countA[path]++
	}

	for _, path := range b {
		countB[path]++
	}

	// Compare the maps
	for path, count := range countA {
		if countB[path] != count {
			return false
		}
	}

	for path, count := range countB {
		if countA[path] != count {
			return false
		}
	}

	return true
}
