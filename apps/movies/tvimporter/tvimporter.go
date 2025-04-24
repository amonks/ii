package tvimporter

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
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
}

func New(tmdb *tmdb.Client, db *db.DB) *TVImporter {
	return &TVImporter{
		tmdb: tmdb,
		db:   db,
	}
}

func (app *TVImporter) Run(ctx context.Context) error {
	log.Println("tvimporter started")
	defer log.Println("tvimporter done")

	// Process the TV import directory
	if err := app.scanTVDirectory(ctx, config.TVImportDir); err != nil {
		return fmt.Errorf("error scanning TV directory: %w", err)
	}

	return nil
}

func (app *TVImporter) scanTVDirectory(ctx context.Context, rootDir string) error {
	// First, identify potential TV show directories
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return fmt.Errorf("error reading TV directory: %w", err)
	}

	// For each possible show directory
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		if !entry.IsDir() {
			continue // Skip files at root level - we're looking for show folders
		}

		showDir := filepath.Join(rootDir, entry.Name())
		log.Printf("Processing potential TV show directory: %s", showDir)

		// Scan the show directory for season directories or episode files
		if err := app.scanShowDirectory(ctx, showDir, entry.Name()); err != nil {
			log.Printf("Error processing show directory %s: %v", showDir, err)
			continue
		}
	}

	return nil
}

func (app *TVImporter) scanShowDirectory(ctx context.Context, showDir, showName string) error {
	entries, err := os.ReadDir(showDir)
	if err != nil {
		return fmt.Errorf("error reading show directory: %w", err)
	}

	// Find season directories or episode files
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		path := filepath.Join(showDir, entry.Name())
		relPath := filepath.Join(showName, entry.Name())

		if entry.IsDir() {
			// Check if this is a season directory
			seasonMatch := seasonFolderPattern.FindStringSubmatch(entry.Name())
			if seasonMatch != nil {
				log.Printf("Found season directory: %s", entry.Name())
				if err := app.scanSeasonDirectory(ctx, path, relPath, showName); err != nil {
					log.Printf("Error processing season directory %s: %v", path, err)
				}
				continue
			}

			// Otherwise, scan this directory for episodes
			if err := app.scanDirectoryForEpisodes(ctx, path, relPath, showName); err != nil {
				log.Printf("Error scanning directory for episodes %s: %v", path, err)
			}
		} else {
			// Check if this is an episode file
			if app.isEpisodeFile(entry.Name()) {
				if err := app.processEpisodeFile(ctx, path, relPath); err != nil {
					log.Printf("Error processing episode file %s: %v", path, err)
				}
			}
		}
	}

	return nil
}

func (app *TVImporter) scanSeasonDirectory(ctx context.Context, seasonDir, relPath, showName string) error {
	return app.scanDirectoryForEpisodes(ctx, seasonDir, relPath, showName)
}

func (app *TVImporter) scanDirectoryForEpisodes(ctx context.Context, dir, relPath, showName string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("error reading directory: %w", err)
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		path := filepath.Join(dir, entry.Name())
		entryRelPath := filepath.Join(relPath, entry.Name())

		if entry.IsDir() {
			// Recursively scan subdirectories
			if err := app.scanDirectoryForEpisodes(ctx, path, entryRelPath, showName); err != nil {
				log.Printf("Error scanning subdirectory %s: %v", path, err)
			}
		} else {
			// Check if this is an episode file
			if app.isEpisodeFile(entry.Name()) {
				if err := app.processEpisodeFile(ctx, path, entryRelPath); err != nil {
					log.Printf("Error processing episode file %s: %v", path, err)
				}
			}
		}
	}

	return nil
}

func (app *TVImporter) isEpisodeFile(filename string) bool {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".mkv" && ext != ".mp4" && ext != ".avi" && ext != ".m4v" {
		return false
	}

	// Check for episode patterns in filename
	lowerName := strings.ToLower(filename)
	return episodePattern.MatchString(lowerName) ||
		seasonEpPattern.MatchString(lowerName) ||
		dotSeasonEpPattern.MatchString(lowerName)
}

func (app *TVImporter) processEpisodeFile(ctx context.Context, fullPath, relPath string) error {
	log.Printf("Processing episode file: %s", relPath)

	// Check if this episode is already in the database or ignored
	if ignored, err := app.db.PathIsIgnored(db.MediaTypeTV, relPath); err != nil {
		return fmt.Errorf("error checking if ignore exists for path '%s': %w", relPath, err)
	} else if ignored {
		return nil
	}

	if exists, err := app.db.TVEpisodeExistsFromPath(relPath); err != nil {
		return fmt.Errorf("error checking if TV episode exists for path '%s': %w", relPath, err)
	} else if exists {
		return nil
	}

	if exists, err := app.db.StubExistsFromPath(db.MediaTypeTV, relPath); err != nil {
		return fmt.Errorf("error checking if stub exists for path '%s': %w", relPath, err)
	} else if exists {
		return nil
	}

	// Create a stub for this episode
	if _, err := app.db.CreateStub(db.MediaTypeTV, relPath); err != nil {
		return fmt.Errorf("error creating stub for path '%s': %w", relPath, err)
	}

	log.Printf("Added TV stub: %s", relPath)
	return nil
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

	return 0, 0, fmt.Errorf("could not parse season and episode from filename: %s", filename)
}