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
	system := system.New("syncer")
	return &MovieSyncer{
		System: system,
		tmdb:   tmdb,
		db:     db,
	}
}

func (app *MovieSyncer) Run(ctx context.Context) error {
	defer app.System.Start()()

	fmt.Println("moviesyncer: start")

	if err := app.tmdb.AuthorizeV4WriteAPI(); err != nil {
		return err
	}

	movies, err := app.db.AllMovies()
	if err != nil {
		return err
	}

	fmt.Printf("%d movies in the library\n", len(movies))

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

	for _, id := range movies {
		if _, has := tmdbSet[id]; !has {
			fmt.Println("adding", id, "to list")
			if err := app.tmdb.AddToList(listID, id); err != nil {
				return err
			}
		}
	}

	fmt.Println("moviesyncer done")

	return nil
}
