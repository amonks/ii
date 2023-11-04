package tvimporter

import (
	"context"
	"os"
	"sync"

	"github.com/pioz/tvdb"
	"monks.co/movietagger/config"
	"monks.co/movietagger/db"
	"monks.co/movietagger/system"
)

func New(tvdb *tvdb.Client, db *db.DB) *TVImporter {
	return &TVImporter{
		System: system.New("tvimporter"),
		db:     db,
		tvdb:   tvdb,
	}
}

type TVImporter struct {
	*system.System
	db   *db.DB
	tvdb *tvdb.Client

	mutex         sync.Mutex
	subscriptions []chan *db.Movie
}

func (app *TVImporter) Run(ctx context.Context) error {
	defer app.System.Start()()

	dir, err := os.ReadDir(config.TVImportDir + "/")
	if err != nil {
		return err
	}
	for _, f := range dir {
		if !f.IsDir() {
			continue
		}
	}

	return nil
}
