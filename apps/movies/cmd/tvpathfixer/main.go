package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
)

type FixResult struct {
	Type      string // "show", "season", "episode"
	OldPath   string
	NewPath   string
	Action    string // "renamed", "would_rename", "verified", "error"
	Error     error
	ShowName  string
	SeasonNum int
	EpisodeID string
}

type FixReport struct {
	TotalShows    int
	TotalSeasons  int
	TotalEpisodes int
	ShowsFixed    int
	SeasonsFixed  int
	EpisodesFixed int
	Verified      int
	Errors        int
	Results       []FixResult
}

func main() {
	dryRun := flag.Bool("dryrun", true, "Run in dry-run mode (no changes made)")
	flag.Parse()

	if *dryRun {
		log.Println("Starting TV path fix (DRY RUN MODE - no changes will be made)...")
	} else {
		log.Println("Starting TV path fix...")
	}

	// Initialize database
	database := db.New(config.DBPath)
	if err := database.Start(); err != nil {
		log.Fatalf("Failed to start database: %v", err)
	}
	defer database.Stop()

	// Get all TV shows
	shows, err := database.AllTVShows()
	if err != nil {
		log.Fatalf("Failed to get TV shows: %v", err)
	}

	log.Printf("Found %d TV shows", len(shows))

	var report FixReport
	report.TotalShows = len(shows)

	// Process each show
	for _, show := range shows {
		// Fix the show directory path
		showResult := fixShowPath(database, show, *dryRun)
		report.Results = append(report.Results, showResult)

		if showResult.Action == "renamed" || showResult.Action == "would_rename" {
			report.ShowsFixed++
		} else if showResult.Action == "verified" {
			report.Verified++
		} else if showResult.Action == "error" {
			report.Errors++
		}

		// Get seasons for this show
		seasons, err := database.GetTVShowSeasons(show.ID)
		if err != nil {
			log.Printf("Error getting seasons for show %s: %v", show.Name, err)
			continue
		}

		report.TotalSeasons += len(seasons)

		// Process each season
		for _, season := range seasons {
			// Fix the season directory path
			seasonResult := fixSeasonPath(database, show, season, *dryRun)
			report.Results = append(report.Results, seasonResult)

			if seasonResult.Action == "renamed" || seasonResult.Action == "would_rename" {
				report.SeasonsFixed++
			} else if seasonResult.Action == "verified" {
				report.Verified++
			} else if seasonResult.Action == "error" {
				report.Errors++
			}

			// Get episodes for this season
			episodes, err := database.GetTVSeasonEpisodes(show.ID, season.SeasonNumber)
			if err != nil {
				log.Printf("Error getting episodes for show %s season %d: %v", show.Name, season.SeasonNumber, err)
				continue
			}

			report.TotalEpisodes += len(episodes)

			// Process each episode
			for _, episode := range episodes {
				episodeResult := fixEpisodePath(database, show, season, episode, *dryRun)
				report.Results = append(report.Results, episodeResult)

				if episodeResult.Action == "renamed" || episodeResult.Action == "would_rename" {
					report.EpisodesFixed++
				} else if episodeResult.Action == "verified" {
					report.Verified++
				} else if episodeResult.Action == "error" {
					report.Errors++
				}
			}
		}
	}

	// Print final report
	printReport(report, *dryRun)
}

func fixShowPath(database *db.DB, show *db.TVShow, dryRun bool) FixResult {
	result := FixResult{
		Type:     "show",
		ShowName: show.Name,
	}

	expectedPath := show.BuildLibraryPath()
	currentPath := show.LibraryPath

	result.OldPath = currentPath
	result.NewPath = expectedPath

	// Check if path needs fixing
	if currentPath == expectedPath {
		result.Action = "verified"
		return result
	}

	// Path needs to be fixed
	oldFullPath := filepath.Join(config.TVLibraryDir, currentPath)
	newFullPath := filepath.Join(config.TVLibraryDir, expectedPath)

	result.Action = "would_rename"
	if !dryRun {
		// Check if the show directory exists yet
		_, err := os.Stat(oldFullPath)
		if os.IsNotExist(err) {
			// Directory doesn't exist yet (no episodes copied) - just update the database
			result.Action = "renamed"

			if err := database.Table("tv_shows").Where("id = ?", show.ID).
				Updates(map[string]any{"library_path": expectedPath}).Error; err != nil {
				result.Action = "error"
				result.Error = fmt.Errorf("failed to update database: %w", err)
				return result
			}

			log.Printf("Updated show path in database (directory not created yet): %s -> %s", currentPath, expectedPath)
			return result
		} else if err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("error checking directory: %w", err)
			return result
		}

		// Directory exists, proceed with rename
		result.Action = "renamed"

		// Check if new directory already exists
		if _, err := os.Stat(newFullPath); err == nil {
			result.Action = "error"
			result.Error = fmt.Errorf("new directory already exists: %s", newFullPath)
			return result
		}

		// Rename the directory
		if err := os.Rename(oldFullPath, newFullPath); err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("failed to rename directory: %w", err)
			return result
		}

		// Update the database
		if err := database.Table("tv_shows").Where("id = ?", show.ID).
			Updates(map[string]any{"library_path": expectedPath}).Error; err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("failed to update database: %w", err)
			return result
		}

		log.Printf("Renamed show directory: %s -> %s", currentPath, expectedPath)
	} else {
		log.Printf("Would rename show directory: %s -> %s", currentPath, expectedPath)
	}

	return result
}

func fixSeasonPath(database *db.DB, show *db.TVShow, season *db.TVSeason, dryRun bool) FixResult {
	result := FixResult{
		Type:      "season",
		ShowName:  show.Name,
		SeasonNum: season.SeasonNumber,
	}

	// Use the show's updated library path to build the expected season path
	expectedPath := season.BuildLibraryPath(show.BuildLibraryPath())
	currentPath := season.LibraryPath

	result.OldPath = currentPath
	result.NewPath = expectedPath

	// Check if path needs fixing
	if currentPath == expectedPath {
		result.Action = "verified"
		return result
	}

	// Path needs to be fixed
	oldFullPath := filepath.Join(config.TVLibraryDir, currentPath)
	newFullPath := filepath.Join(config.TVLibraryDir, expectedPath)

	result.Action = "would_rename"
	if !dryRun {
		// Check if the season directory exists yet
		_, err := os.Stat(oldFullPath)
		if os.IsNotExist(err) {
			// Directory doesn't exist yet (no episodes copied) - just update the database
			result.Action = "renamed"

			if err := database.Table("tv_seasons").
				Where("show_id = ? AND season_number = ?", show.ID, season.SeasonNumber).
				Updates(map[string]any{"library_path": expectedPath}).Error; err != nil {
				result.Action = "error"
				result.Error = fmt.Errorf("failed to update database: %w", err)
				return result
			}

			log.Printf("Updated season path in database (directory not created yet): %s -> %s", currentPath, expectedPath)
			return result
		} else if err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("error checking directory: %w", err)
			return result
		}

		// Directory exists, proceed with rename
		result.Action = "renamed"

		// Check if new directory already exists
		if _, err := os.Stat(newFullPath); err == nil {
			result.Action = "error"
			result.Error = fmt.Errorf("new directory already exists: %s", newFullPath)
			return result
		}

		// Ensure parent directory exists
		parentDir := filepath.Dir(newFullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("failed to create parent directory: %w", err)
			return result
		}

		// Rename the directory
		if err := os.Rename(oldFullPath, newFullPath); err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("failed to rename directory: %w", err)
			return result
		}

		// Update the database
		if err := database.Table("tv_seasons").
			Where("show_id = ? AND season_number = ?", show.ID, season.SeasonNumber).
			Updates(map[string]any{"library_path": expectedPath}).Error; err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("failed to update database: %w", err)
			return result
		}

		log.Printf("Renamed season directory: %s -> %s", currentPath, expectedPath)
	} else {
		log.Printf("Would rename season directory: %s -> %s", currentPath, expectedPath)
	}

	return result
}

func fixEpisodePath(database *db.DB, show *db.TVShow, season *db.TVSeason, episode *db.TVEpisode, dryRun bool) FixResult {
	result := FixResult{
		Type:      "episode",
		ShowName:  show.Name,
		SeasonNum: season.SeasonNumber,
		EpisodeID: fmt.Sprintf("S%02dE%02d", episode.SeasonNumber, episode.EpisodeNumber),
	}

	// Use the season's CURRENT library path
	expectedPath := episode.BuildLibraryPath(season.BuildLibraryPath(show.BuildLibraryPath()))
	currentPath := episode.LibraryPath

	result.OldPath = currentPath
	result.NewPath = expectedPath

	// Check if path needs fixing
	if currentPath == expectedPath {
		result.Action = "verified"
		return result
	}

	// Path needs to be fixed
	oldFullPath := filepath.Join(config.TVLibraryDir, currentPath)
	newFullPath := filepath.Join(config.TVLibraryDir, expectedPath)

	result.Action = "would_rename"
	if !dryRun {
		// Check if the episode has been copied to the library yet
		_, err := os.Stat(oldFullPath)
		if os.IsNotExist(err) {
			// File hasn't been copied yet - just update the database path
			result.Action = "renamed"

			if err := database.Table("tv_episodes").
				Where("show_id = ? AND season_number = ? AND episode_number = ?",
					show.ID, episode.SeasonNumber, episode.EpisodeNumber).
				Updates(map[string]any{"library_path": expectedPath}).Error; err != nil {
				result.Action = "error"
				result.Error = fmt.Errorf("failed to update database: %w", err)
				return result
			}

			log.Printf("Updated uncoped episode path in database: %s -> %s", currentPath, expectedPath)
			return result
		} else if err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("error checking file: %w", err)
			return result
		}

		// File exists, proceed with rename
		result.Action = "renamed"

		// Check if new file already exists
		if _, err := os.Stat(newFullPath); err == nil {
			result.Action = "error"
			result.Error = fmt.Errorf("new file already exists: %s", newFullPath)
			return result
		}

		// Ensure parent directory exists
		parentDir := filepath.Dir(newFullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("failed to create parent directory: %w", err)
			return result
		}

		// Rename the file
		if err := os.Rename(oldFullPath, newFullPath); err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("failed to rename file: %w", err)
			return result
		}

		// Update the database
		if err := database.Table("tv_episodes").
			Where("show_id = ? AND season_number = ? AND episode_number = ?",
				show.ID, episode.SeasonNumber, episode.EpisodeNumber).
			Updates(map[string]any{"library_path": expectedPath}).Error; err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("failed to update database: %w", err)
			return result
		}

		log.Printf("Renamed episode file: %s -> %s", currentPath, expectedPath)
	} else {
		log.Printf("Would rename episode file: %s -> %s", currentPath, expectedPath)
	}

	return result
}

func printReport(report FixReport, dryRun bool) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	if dryRun {
		fmt.Println("TV PATH FIX REPORT (DRY RUN)")
	} else {
		fmt.Println("TV PATH FIX REPORT")
	}
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total shows: %d\n", report.TotalShows)
	fmt.Printf("Total seasons: %d\n", report.TotalSeasons)
	fmt.Printf("Total episodes: %d\n", report.TotalEpisodes)
	fmt.Println(strings.Repeat("-", 60))

	if dryRun {
		fmt.Printf("Shows that would be fixed: %d\n", report.ShowsFixed)
		fmt.Printf("Seasons that would be fixed: %d\n", report.SeasonsFixed)
		fmt.Printf("Episodes that would be fixed: %d\n", report.EpisodesFixed)
	} else {
		fmt.Printf("Shows fixed: %d\n", report.ShowsFixed)
		fmt.Printf("Seasons fixed: %d\n", report.SeasonsFixed)
		fmt.Printf("Episodes fixed: %d\n", report.EpisodesFixed)
	}

	fmt.Printf("Already correct (verified): %d\n", report.Verified)
	fmt.Printf("Errors: %d\n", report.Errors)
	fmt.Println(strings.Repeat("=", 60))

	// Print detailed results
	if report.ShowsFixed > 0 {
		if dryRun {
			fmt.Println("\nSHOWS THAT WOULD BE RENAMED:")
		} else {
			fmt.Println("\nSHOWS RENAMED:")
		}
		for _, result := range report.Results {
			if result.Type == "show" && (result.Action == "renamed" || result.Action == "would_rename") {
				fmt.Printf("  %s\n", result.ShowName)
				fmt.Printf("    Old: %s\n", result.OldPath)
				fmt.Printf("    New: %s\n", result.NewPath)
			}
		}
	}

	if report.SeasonsFixed > 0 {
		if dryRun {
			fmt.Println("\nSEASONS THAT WOULD BE RENAMED:")
		} else {
			fmt.Println("\nSEASONS RENAMED:")
		}
		for _, result := range report.Results {
			if result.Type == "season" && (result.Action == "renamed" || result.Action == "would_rename") {
				fmt.Printf("  %s - Season %d\n", result.ShowName, result.SeasonNum)
				fmt.Printf("    Old: %s\n", result.OldPath)
				fmt.Printf("    New: %s\n", result.NewPath)
			}
		}
	}

	if report.EpisodesFixed > 0 {
		if dryRun {
			fmt.Println("\nEPISODES THAT WOULD BE RENAMED:")
		} else {
			fmt.Println("\nEPISODES RENAMED:")
		}
		for _, result := range report.Results {
			if result.Type == "episode" && (result.Action == "renamed" || result.Action == "would_rename") {
				fmt.Printf("  %s - %s\n", result.ShowName, result.EpisodeID)
				fmt.Printf("    Old: %s\n", result.OldPath)
				fmt.Printf("    New: %s\n", result.NewPath)
			}
		}
	}

	if report.Errors > 0 {
		fmt.Println("\nERRORS:")
		for _, result := range report.Results {
			if result.Action == "error" {
				fmt.Printf("  [%s] %s", result.Type, result.ShowName)
				if result.Type == "season" {
					fmt.Printf(" - Season %d", result.SeasonNum)
				} else if result.Type == "episode" {
					fmt.Printf(" - %s", result.EpisodeID)
				}
				fmt.Printf("\n    Error: %v\n", result.Error)
			}
		}
	}
}
