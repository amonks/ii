package moviesyncer

import (
	"context"
	"fmt"
	"sync"

	"monks.co/movietagger/db"
	"monks.co/movietagger/system"
	"monks.co/movietagger/tmdb"
)

type MovieSyncer struct {
	*system.System
	tmdb  *tmdb.Client
	db    *db.DB
	mutex sync.Mutex
}

func New(tmdb *tmdb.Client, db *db.DB) *MovieSyncer {
	system := system.New("moviesyncer")
	return &MovieSyncer{
		System: system,
		tmdb:   tmdb,
		db:     db,
	}
}

func (app *MovieSyncer) Run(ctx context.Context) error {
	defer app.System.Start()()

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

	fmt.Printf("%d movies in the tmdb list\n", len(tmdbList))

	for _, movie := range movies {
		if _, has := tmdbSet[movie.ID]; !has {
			fmt.Println("adding", movie.ID, "to list")
			if err := app.tmdb.AddToList(listID, movie.ID); err != nil {
				return err
			}
		}
	}

	return nil
}
