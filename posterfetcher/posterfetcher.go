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
	system := system.New("fetcher")
	return &PosterFetcher{
		System: system,
		tmdb:   tmdb,
		db:     db,
	}
}

func (app *PosterFetcher) Run(ctx context.Context) error {
	defer app.System.Start()()

	fmt.Println("posterfetcher: start")

	movies, err := app.db.AllMovies()
	if err != nil {
		return err
	}

	fmt.Printf("%d movies in the library\n", len(movies))

	for _, id := range movies {
		movie, err := app.db.GetMovie(id)
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

		fmt.Println("fetching poster for", id, movie.Title)

		posterPath := path.Clean("/mypool/tank/movies/posters/" + tmdbJSON.PosterPath)
		f, err := os.Create(posterPath)
		if err != nil {
			return err
		}
		resp, err := http.Get("https://image.tmdb.org/t/p/original" + tmdbJSON.PosterPath)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if _, err := io.Copy(f, resp.Body); err != nil {
			return err
		}

		if err := app.db.AddMoviePoster(id, posterPath); err != nil {
			return fmt.Errorf("error updating %d (%s): %w", id, movie.Title, err)
		}
	}

	fmt.Println("posterfetcher done")

	return nil
}
