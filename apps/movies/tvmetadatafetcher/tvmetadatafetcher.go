package tvmetadatafetcher

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"monks.co/apps/movies/db"
	"monks.co/pkg/tmdb"
)

type TVMetadataFetcher struct {
	mu   sync.Mutex
	tmdb *tmdb.Client
	db   *db.DB
}

func New(tmdb *tmdb.Client, db *db.DB) *TVMetadataFetcher {
	return &TVMetadataFetcher{
		tmdb: tmdb,
		db:   db,
	}
}

func (app *TVMetadataFetcher) Run(ctx context.Context) error {
	app.mu.Lock()
	defer app.mu.Unlock()

	log.Println("tvmetadatafetcher started")
	defer log.Println("tvmetadatafetcher done")

	shows, err := app.db.AllTVShows()
	if err != nil {
		return fmt.Errorf("error getting TV shows: %w", err)
	}

	for _, show := range shows {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Fetch additional show metadata if needed
		if show.PosterPath == "" {
			log.Printf("Fetching poster for TV show: %s", show.Name)
			if err := app.fetchShowPoster(show); err != nil {
				log.Printf("Error fetching poster for TV show %s: %v", show.Name, err)
			}
		}

		// For each season, fetch metadata
		for _, season := range show.Seasons {
			if err := ctx.Err(); err != nil {
				return err
			}

			// Fetch season poster if needed
			if season.PosterPath == "" {
				log.Printf("Fetching poster for season %d of %s", season.SeasonNumber, show.Name)
				if err := app.fetchSeasonPoster(show.ID, season.SeasonNumber); err != nil {
					log.Printf("Error fetching poster for season %d of %s: %v", season.SeasonNumber, show.Name, err)
				}
			}
		}
	}

	return nil
}

func (app *TVMetadataFetcher) fetchShowPoster(show *db.TVShow) error {
	// Fetch full TV show details to get poster path
	tmdbShow, err := app.tmdb.GetTV(show.ID)
	if err != nil {
		return fmt.Errorf("error fetching TV show details: %w", err)
	}

	if tmdbShow.PosterPath != "" {
		posterURL := fmt.Sprintf("https://image.tmdb.org/t/p/original%s", tmdbShow.PosterPath)
		posterPath := filepath.Join("/data/tank/tv", "posters", fmt.Sprintf("show_%d.jpg", show.ID))

		// Download poster
		if err := downloadImage(posterURL, posterPath); err != nil {
			return fmt.Errorf("error downloading poster: %w", err)
		}

		// Update database with poster path
		if err := app.db.AddTVShowPoster(show, posterPath); err != nil {
			return fmt.Errorf("error updating TV show poster path: %w", err)
		}
	}

	return nil
}

func (app *TVMetadataFetcher) fetchSeasonPoster(showID int64, seasonNumber int) error {
	// Get season details
	season, _, err := app.tmdb.GetSeason(showID, seasonNumber)
	if err != nil {
		return fmt.Errorf("error fetching season details: %w", err)
	}

	if season.PosterPath != "" {
		posterURL := fmt.Sprintf("https://image.tmdb.org/t/p/original%s", season.PosterPath)
		posterPath := filepath.Join("/data/tank/tv", "posters", fmt.Sprintf("season_%d_%d.jpg", showID, seasonNumber))

		// Download poster
		if err := downloadImage(posterURL, posterPath); err != nil {
			return fmt.Errorf("error downloading season poster: %w", err)
		}

		// Update database with poster path
		if err := app.db.AddTVSeasonPoster(showID, seasonNumber, posterPath); err != nil {
			return fmt.Errorf("error updating season poster path: %w", err)
		}
	}

	return nil
}

// Helper function to download an image from a URL
func downloadImage(url, destPath string) error {
	// Create the destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	// Download the image
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error downloading image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error downloading image, status code: %d", resp.StatusCode)
	}

	// Create the destination file
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer f.Close()

	// Copy the image data to the file
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("error saving image: %w", err)
	}

	return nil
}