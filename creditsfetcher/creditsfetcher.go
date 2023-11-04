package creditsfetcher

import (
	"context"
	"fmt"
	"sync"

	"monks.co/movietagger/db"
	"monks.co/movietagger/system"
	"monks.co/movietagger/tmdb"
)

type CreditsFetcher struct {
	*system.System
	tmdb  *tmdb.Client
	db    *db.DB
	mutex sync.Mutex
}

func New(tmdb *tmdb.Client, db *db.DB) *CreditsFetcher {
	system := system.New("creditsfetcher")
	return &CreditsFetcher{
		System: system,
		tmdb:   tmdb,
		db:     db,
	}
}

func (app *CreditsFetcher) Run(ctx context.Context) error {
	defer app.System.Start().Stop()

	movies, err := app.db.AllMovies()
	if err != nil {
		return err
	}

	for _, movie := range movies {
		if len(movie.TMDBCreditsJSON) != 0 {
			continue
		}

		fmt.Println("fetching credits for", movie.ID, movie.Title)

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
	}

	return nil
}
