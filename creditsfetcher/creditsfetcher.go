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
	defer app.System.Start()()

	fmt.Println("creditsfetcher: start")

	movies, err := app.db.AllMovies()
	if err != nil {
		return err
	}

	for _, id := range movies {
		movie, err := app.db.GetMovie(id)
		if len(movie.TMDBCreditsJSON) != 0 {
			continue
		}

		fmt.Println("fetching credits for", id, movie.Title)

		credits, creditsJSON, err := app.tmdb.GetCredits(id)
		if err != nil {
			return fmt.Errorf("error fetching %d (%s): %w", id, movie.Title, err)
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
			if err := app.db.AddMovieDirector(id, director); err != nil {
				return fmt.Errorf("error updating %d (%s): %w", id, movie.Title, err)
			}
		}
		if writer != "" {
			if err := app.db.AddMovieWriter(id, writer); err != nil {
				return fmt.Errorf("error updating %d (%s): %w", id, movie.Title, err)
			}
		}
		if err := app.db.AddMovieCredits(id, creditsJSON); err != nil {
			return fmt.Errorf("error updating %d (%s): %w", id, movie.Title, err)
		}
	}

	fmt.Println("creditsfetcher done")

	return nil
}
