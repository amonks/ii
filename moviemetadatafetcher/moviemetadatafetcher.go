package moviemetadatafetcher

import (
	"context"
	"fmt"
	"log"
	"sync"

	"monks.co/movietagger/db"
	"monks.co/movietagger/tmdb"
)

type MovieMetadataFetcher struct {
	tmdb  *tmdb.Client
	db    *db.DB
	mutex sync.Mutex
}

func New(tmdb *tmdb.Client, db *db.DB) *MovieMetadataFetcher {
	return &MovieMetadataFetcher{
		tmdb:   tmdb,
		db:     db,
	}
}

func (app *MovieMetadataFetcher) Run(ctx context.Context) error {
	log.Println("moviemetadatafetcher started")
	defer log.Println("moviemetadatafetcher done")

	movies, err := app.db.AllMovies()
	if err != nil {
		return err
	}

	for _, movie := range movies {
		if len(movie.TMDBJSON) != 0 {
			continue
		}

		log.Println("fetching json for", movie.ID, movie.Title)

		tmdbMovie, err := app.tmdb.Get(movie.ID)
		if err != nil {
			return fmt.Errorf("error fetching %d (%s): %w", movie.ID, movie.Title, err)
		}
		if err := app.db.AddMovieJSON(movie, tmdbMovie.TMDBJSON); err != nil {
			return fmt.Errorf("error updating %d (%s): %w", movie.ID, movie.Title, err)
		}
	}

	return nil
}
