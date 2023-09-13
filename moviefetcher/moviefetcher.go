package moviefetcher

import (
	"context"
	"fmt"
	"sync"

	"monks.co/movietagger/db"
	"monks.co/movietagger/system"
	"monks.co/movietagger/tmdb"
)

type MovieFetcher struct {
	*system.System
	tmdb  *tmdb.Client
	db    *db.DB
	mutex sync.Mutex
}

func New(tmdb *tmdb.Client, db *db.DB) *MovieFetcher {
	system := system.New("fetcher")
	return &MovieFetcher{
		System: system,
		tmdb:   tmdb,
		db:     db,
	}
}

func (app *MovieFetcher) Run(ctx context.Context) error {
	defer app.System.Start()()

	fmt.Println("moviefetcher: start")

	movies, err := app.db.AllMovies()
	if err != nil {
		return err
	}

	fmt.Printf("%d movies in the library\n", len(movies))

	for _, id := range movies {
		movie, err := app.db.GetMovie(id)
		if len(movie.TMDBJSON) != 0 {
			continue
		}

		fmt.Println("fetching json for", id, movie.Title)

		tmdbMovie, err := app.tmdb.Get(id)
		if err != nil {
			return fmt.Errorf("error fetching %d (%s): %w", id, movie.Title, err)
		}
		fmt.Println(id, string(tmdbMovie.TMDBJSON))
		if err := app.db.AddMovieJSON(id, tmdbMovie.TMDBJSON); err != nil {
			return fmt.Errorf("error updating %d (%s): %w", id, movie.Title, err)
		}
	}

	fmt.Println("moviefetcher done")

	return nil
}
