package ratingfetcher

import (
	"context"
	"fmt"
	"log"
	"time"

	"monks.co/apps/movies/db"
	"monks.co/pkg/metacritic"
)

type RatingFetcher struct {
	db *db.DB
}

func New(db *db.DB) *RatingFetcher {
	return &RatingFetcher{
		db: db,
	}
}

func (app *RatingFetcher) Run(ctx context.Context) error {
	movies, err := app.db.AllMovies()
	if err != nil {
		return err
	}

	for _, movie := range movies {
		if movie.MetacriticValidated {
			continue
		}
		if movie.MetacriticRating != 0 {
			continue
		}
		results, err := metacritic.SearchMovies(movie.Title)
		if err != nil {
			log.Println(fmt.Errorf("error looking for '%s': %w", movie.Title, err))
			continue
		}
		if len(results) == 0 {
			log.Printf("no results for '%s'", movie.Title)
			continue
		}
		if err := app.db.AddMovieRating(movie, results[0].Score, results[0].URL); err != nil {
			return err
		}
		log.Printf("fetched rating for '%s': %d [%s]", movie.Title, results[0].Score, results[0].URL)

		<-time.After(time.Second)
	}

	return nil
}
