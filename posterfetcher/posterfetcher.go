package posterfetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sync"

	"monks.co/movietagger/db"
	"monks.co/movietagger/system"
	"monks.co/movietagger/tmdb"
)

type PosterFetcher struct {
	*system.System
	tmdb  *tmdb.Client
	db    *db.DB
	mutex sync.Mutex
}

func New(tmdb *tmdb.Client, db *db.DB) *PosterFetcher {
	system := system.New("posterfetcher")
	return &PosterFetcher{
		System: system,
		tmdb:   tmdb,
		db:     db,
	}
}

func (app *PosterFetcher) Run(ctx context.Context) error {
	defer app.System.Start().Stop()

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

		fmt.Println("fetching poster for", movie.ID, movie.Title)

		app.Println("creating file")
		posterPath := path.Clean("/data/tank/movies/posters/" + tmdbJSON.PosterPath)
		f, err := os.Create(posterPath)
		if err != nil {
			return err
		}

		app.Println("fetching poster")
		resp, err := http.Get("https://image.tmdb.org/t/p/original" + tmdbJSON.PosterPath)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		app.Println("copying poster data")
		if _, err := io.Copy(f, resp.Body); err != nil {
			return err
		}

		app.Println("adding movie poster")
		if err := app.db.AddMoviePoster(movie, posterPath); err != nil {
			return fmt.Errorf("error updating %d (%s): %w", movie.ID, movie.Title, err)
		}
	}

	return nil
}
