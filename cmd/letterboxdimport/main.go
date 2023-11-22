package main

import (
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
}

func run() error {
	db := db.New(config.DBPath)
	if err := db.Start(); err != nil {
		return err
	}
	defer db.Stop()

	if err := db.AutoMigrate(letterboxd.Watch{}); err != nil {
		return err
	}

	entries, err := letterboxd.FetchDiary("amonks", 1, 10)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fmt.Println("adding", entry.MovieTitle)
		if _, err := db.CreateWatch(entry); err != nil {
			if !strings.Contains(err.Error(), "UNIQUE constraint failed: watches.letterboxd_url") {
				return err
			}
		}
	}

	return nil
}
