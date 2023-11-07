package tvimporter

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/pioz/tvdb"
	"monks.co/movietagger/config"
	"monks.co/movietagger/db"
)

func New(tvdb *tvdb.Client, db *db.DB) *TVImporter {
	return &TVImporter{
		db:   db,
		tvdb: tvdb,
	}
}

type TVImporter struct {
	db   *db.DB
	tvdb *tvdb.Client

	mutex         sync.Mutex
	subscriptions []chan *db.Movie
}

func (app *TVImporter) Run(ctx context.Context) error {
	log.Printf("tvimporter started")
	defer log.Printf("tvimporter done")

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
