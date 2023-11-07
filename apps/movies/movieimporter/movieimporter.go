package movieimporter

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
	"monks.co/pkg/tmdb"
)

func New(tmdb *tmdb.Client, db *db.DB) *MovieImporter {
	return &MovieImporter{
		tmdb: tmdb,
		db:   db,
	}
}

type MovieImporter struct {
	tmdb *tmdb.Client
	db   *db.DB
}

func (app *MovieImporter) Run(ctx context.Context) error {
	log.Println("movieimporter started")
	defer log.Println("movieimporter done")

	if err := filepath.Walk(config.MovieImportDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Println("Walk passed an error into callback:", err)
			return err
		}
		if err := ctx.Err(); err != nil {
			log.Printf("canceled")
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		path = strings.TrimPrefix(path, config.MovieImportDir+"/")

		if !strings.HasSuffix(path, "mkv") {
			return nil
		}

		if ignored, err := app.db.PathIsIgnored(path); err != nil {
			log.Printf("err checking ignore")
			return err
		} else if ignored {
			return nil
		}

		if exists, err := app.db.StubExistsFromPath(path); err != nil {
			log.Printf("err checking exists")
			return err
		} else if exists {
			return nil
		}

		if exists, err := app.db.MovieExistsFromPath(path); err != nil {
			log.Printf("err checking exists")
			return err
		} else if exists {
			return nil
		}

		if _, err := app.db.CreateStub(path); err != nil {
			return fmt.Errorf("error saving new stub: %w", err)
		}

		log.Printf("added stub: %s", path)

		return nil
	}); err != nil {
		// If a file is deleted between the readdir and the lstat,
		// filepath.Walk exits with an error. That's fine; we must be
		// downloading torrents. Just exit successfully and expect to
		// be retried eventually.
		if str := err.Error(); strings.HasPrefix(str, "lstat") && strings.HasSuffix(str, "no such file or directory") {
			return nil
		}
		return err
	}

	return nil
}
