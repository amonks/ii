package tvimporter

import (
	"context"
	"fmt"
	"log"
	"os"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
	"monks.co/pkg/tmdb"
)

func New(tmdb *tmdb.Client, db *db.DB) *TVImporter {
	return &TVImporter{
		tmdb: tmdb,
		db:   db,
	}
}

type TVImporter struct {
	tmdb *tmdb.Client
	db   *db.DB
}

func (app *TVImporter) Run(ctx context.Context) error {
	log.Println("tvimporter started")
	defer log.Println("tvimporter done")

	dir, err := os.ReadDir(config.TVImportDir + "/")
	if err != nil {
		return err
	}
	for _, f := range dir {
		if err := ctx.Err(); err != nil {
			log.Printf("canceled")
			return err
		}

		if !f.IsDir() {
			continue
		}

		path := f.Name()
		if ignored, err := app.db.PathIsIgnored(db.MediaTypeTV, path); err != nil {
			log.Printf("err checking ignore")
			return err
		} else if ignored {
			return nil
		}

		if exists, err := app.db.StubExistsFromPath(db.MediaTypeTV, path); err != nil {
			log.Printf("err checking exists")
			return err
		} else if exists {
			return nil
		}

		if exists, err := app.db.TVSeriesExistsFromPath(path); err != nil {
			log.Printf("err checking exists")
			return err
		} else if exists {
			return nil
		}

		if _, err := app.db.CreateTVSeriesStub(path); err != nil {
			return fmt.Errorf("error saving new stub: %w", err)
		}

		log.Printf("added stub: %s", path)
	}

	return nil
}
