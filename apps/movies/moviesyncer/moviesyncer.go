package moviesyncer

import (
	"context"
	"log"
	"sync"

	"monks.co/apps/movies/db"
	"monks.co/pkg/tmdb"
)

type MovieSyncer struct {
	tmdb  *tmdb.Client
	db    *db.DB
	mutex sync.Mutex
}

func New(tmdb *tmdb.Client, db *db.DB) *MovieSyncer {
	return &MovieSyncer{
		tmdb: tmdb,
		db:   db,
	}
}

func (app *MovieSyncer) Run(ctx context.Context) error {
	log.Printf("moviesyncer started")
	defer log.Printf("moviesyncer done")

	if err := app.tmdb.AuthorizeV4WriteAPI(); err != nil {
		return err
	}

	movies, err := app.db.AllMovies()
	if err != nil {
		return err
	}

	const listID = 8269679

	tmdbList, err := app.tmdb.List(listID)
	if err != nil {
		return err
	}
	tmdbSet := map[int64]struct{}{}
	for _, m := range tmdbList {
		tmdbSet[m.ID] = struct{}{}
	}

	log.Printf("%d movies in the tmdb list\n", len(tmdbList))

	for _, movie := range movies {
		if _, has := tmdbSet[movie.ID]; !has {
			log.Println("adding", movie.ID, "to list")
			if err := app.tmdb.AddToList(listID, movie.ID); err != nil {
				return err
			}
		}
	}

	return nil
}
