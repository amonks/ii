package letterboxdimporter

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"monks.co/apps/movies/db"
	"monks.co/pkg/letterboxd"
)

type LetterboxdImporter struct {
	db *db.DB
}

func New(db *db.DB) *LetterboxdImporter {
	return &LetterboxdImporter{db: db}
}

var ErrDuplicate = fmt.Errorf("duplicate")

func (li *LetterboxdImporter) Run() error {
	log.Println("letterboxdimporter started")
	err := letterboxd.FetchDiary("amonks", 1, 10, func(entry *letterboxd.Watch) error {
		fmt.Println("adding", entry.MovieTitle)
		if _, err := li.db.CreateWatch(entry); err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed: watches.letterboxd_url") {
				return ErrDuplicate
			}
			return err
		}
		if movie, err := li.db.FindMovieByTitle(entry.MovieTitle); err != nil {
			return err
		} else if movie != nil {
			log.Printf("removing '%s' from queue", entry.MovieTitle)
			if err := li.db.RemoveFromQueue(movie.ID); err != nil {
				return err
			}
		} else {
			log.Printf("could not find movie '%s' for queue removal", entry.MovieTitle)
		}
		return nil
	})
	if err != nil && !errors.Is(err, ErrDuplicate) {
		return err
	}
	log.Println("letterboxdimporter done")
	return nil
}
