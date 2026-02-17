package posterfetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"

	"monks.co/apps/movies/db"
	"monks.co/pkg/tmdb"
)

type PosterFetcher struct {
	tmdb *tmdb.Client
	db   *db.DB
}

func New(tmdb *tmdb.Client, db *db.DB) *PosterFetcher {
	return &PosterFetcher{
		tmdb: tmdb,
		db:   db,
	}
}

func (app *PosterFetcher) Run(ctx context.Context) error {
	movies, err := app.db.AllMovies()
	if err != nil {
		return err
	}

	for _, movie := range movies {
		if len(movie.PosterPath) != 0 {
			continue
		}

		var tmdbJSON struct {
			PosterPath string `json:"poster_path"`
		}
		json.Unmarshal([]byte(movie.TMDBJSON), &tmdbJSON)
		if len(tmdbJSON.PosterPath) == 0 {
			continue
		}

		log.Println("fetching poster for", movie.ID, movie.Title)

		log.Println("creating file")
		posterPath := path.Clean("/data/tank/movies/posters/" + tmdbJSON.PosterPath)
		f, err := os.Create(posterPath)
		if err != nil {
			return err
		}

		log.Println("fetching poster")
		resp, err := http.Get("https://image.tmdb.org/t/p/original" + tmdbJSON.PosterPath)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		log.Println("copying poster data")
		if _, err := io.Copy(f, resp.Body); err != nil {
			return err
		}

		log.Println("adding movie poster")
		if err := app.db.AddMoviePoster(movie, posterPath); err != nil {
			return fmt.Errorf("error updating %d (%s): %w", movie.ID, movie.Title, err)
		}

		log.Println("fetched poster for", movie.ID, movie.Title)
	}

	return nil
}
