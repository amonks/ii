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
		return nil
	})
	if err != nil && !errors.Is(err, ErrDuplicate) {
		return err
	}
	log.Println("letterboxdimporter done")
	return nil
}
