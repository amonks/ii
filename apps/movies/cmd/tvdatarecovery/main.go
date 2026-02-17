package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("TV Data Recovery starting...")

	// Initialize database
	moviesDB := db.New(config.DBPath)
	if err := moviesDB.Start(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer moviesDB.Stop()

	// Get all TV shows
	shows, err := moviesDB.AllTVShows()
	if err != nil {
		log.Fatalf("Failed to fetch TV shows: %v", err)
	}

	log.Printf("Processing %d TV shows", len(shows))

	postersFixed := 0
	episodesFixed := 0

	// Process each show
	for _, show := range shows {
		// Check show poster
		if show.PosterPath != "" {
			if _, err := os.Stat(show.PosterPath); os.IsNotExist(err) {
				log.Printf("Missing poster for show: %s (ID: %d)", show.Name, show.ID)
				// Reset poster path in database to trigger re-download
				if err := moviesDB.AddTVShowPoster(show, ""); err != nil {
					log.Printf("Error resetting poster path for show %s: %v", show.Name, err)
				} else {
					postersFixed++
				}
			}
		}

		// Process each season
		for _, season := range show.Seasons {
			// Get episodes for this season
			episodes, err := moviesDB.GetTVSeasonEpisodes(show.ID, season.SeasonNumber)
			if err != nil {
				log.Printf("Error fetching episodes for season %d of %s: %v", season.SeasonNumber, show.Name, err)
				continue
			}

			// Process each episode
			for _, episode := range episodes {
				if episode.IsCopied {
					destPath := filepath.Join(config.TVLibraryDir, episode.LibraryPath)
					if _, err := os.Stat(destPath); os.IsNotExist(err) {
						log.Printf("Missing episode file: S%02dE%02d - %s of %s",
							episode.SeasonNumber, episode.EpisodeNumber, episode.Name, show.Name)

						// Reset copied status in database to trigger re-copy
						if err := resetEpisodeCopiedStatus(moviesDB, episode); err != nil {
							log.Printf("Error resetting copied status for episode S%02dE%02d of %s: %v",
								episode.SeasonNumber, episode.EpisodeNumber, show.Name, err)
						} else {
							episodesFixed++
						}
					}
				}
			}
		}
	}

	log.Printf("Recovery completed. Reset %d posters and %d episodes for re-processing.", postersFixed, episodesFixed)
}

// Helper function to reset episode copied status
func resetEpisodeCopiedStatus(db *db.DB, episode *db.TVEpisode) error {
	// Create a new episode with the same data but IsCopied set to false
	updatedEpisode := *episode
	updatedEpisode.IsCopied = false

	// Update the episode in the database
	err := db.Table("tv_episodes").
		Where("show_id = ? AND season_number = ? AND episode_number = ?",
			episode.ShowID, episode.SeasonNumber, episode.EpisodeNumber).
		Updates(map[string]any{"is_copied": false}).Error

	if err != nil {
		return fmt.Errorf("failed to update episode: %w", err)
	}

	return nil
}
