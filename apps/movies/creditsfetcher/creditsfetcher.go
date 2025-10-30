package creditsfetcher

import (
	"context"
	"fmt"
	"log"
	"sync"

	"monks.co/apps/movies/db"
	"monks.co/pkg/tmdb"
)

type CreditsFetcher struct {
	tmdb  *tmdb.Client
	db    *db.DB
	mutex sync.Mutex
}

func New(tmdb *tmdb.Client, db *db.DB) *CreditsFetcher {
	return &CreditsFetcher{
		tmdb: tmdb,
		db:   db,
	}
}

func (app *CreditsFetcher) Run(ctx context.Context) error {
	log.Printf("creditsfetcher started")
	defer log.Printf("creditsfetcher done")

	movies, err := app.db.AllMovies()
	if err != nil {
		return err
	}

	for _, movie := range movies {
		if len(movie.TMDBCreditsJSON) != 0 {
			continue
		}

		log.Println("fetching credits for", movie.ID, movie.Title)

		credits, creditsJSON, err := app.tmdb.GetCredits(movie.ID)
		if err != nil {
			return fmt.Errorf("error fetching %d (%s): %w", movie.ID, movie.Title, err)
		}
		var director string
		for _, person := range credits.Crew {
			if person.Job == "Director" {
				director = person.Name
				break
			}
		}
		var writer string
		for _, person := range credits.Crew {
			if person.Job == "Writer" {
				writer = person.Name
				break
			}
		}
		if director != "" {
			if err := app.db.AddMovieDirector(movie, director); err != nil {
				return fmt.Errorf("error updating %d (%s): %w", movie.ID, movie.Title, err)
			}
		}
		if writer != "" {
			if err := app.db.AddMovieWriter(movie, writer); err != nil {
				return fmt.Errorf("error updating %d (%s): %w", movie.ID, movie.Title, err)
			}
		}
		if err := app.db.AddMovieCredits(movie, creditsJSON); err != nil {
			return fmt.Errorf("error updating %d (%s): %w", movie.ID, movie.Title, err)
		}

		log.Println("fetched credits for", movie.ID, movie.Title)
	}

	return nil
}
