package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
	"monks.co/pkg/letterboxd"
)

func main() {
	if err := run(); err != nil {
		log.Printf("stopped: %s\n", err)
		os.Exit(1)
	}

	log.Printf("done")
}

var errDuplicate = fmt.Errorf("duplicate")

func run() error {
	db := db.New(config.DBPath)
	if err := db.Start(); err != nil {
		return err
	}
	defer db.Stop()

	if err := db.AutoMigrate(letterboxd.Watch{}); err != nil {
		return err
	}

	err := letterboxd.FetchDiary("amonks", 1, 10, func(entry *letterboxd.Watch) error {
		fmt.Println("adding", entry.MovieTitle)
		if _, err := db.CreateWatch(entry); err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed: watches.letterboxd_url") {
				return errDuplicate
			}
			return err
		}
		return nil
	})
	if err != nil && !errors.Is(err, errDuplicate) {
		return err
	}

	return nil
}
