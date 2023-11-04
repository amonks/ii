package main

import (
	"context"
	"fmt"

	"monks.co/movietagger/config"
	"monks.co/movietagger/db"
	"monks.co/movietagger/moviefetcher"
	"monks.co/movietagger/tmdb"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	// how to get a token from an api key:
	// http://dev.travisbell.com/play/v4_auth.html
	tmdb := tmdb.New(
		"88f973483e2dc73cfb5053bc059ae33b",
		"eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiI4OGY5NzM0ODNlMmRjNzNjZmI1MDUzYmMwNTlhZTMzYiIsInN1YiI6IjYzZjQ1ZWVkY2FhY2EyMDBhMTljZmQ5OCIsInNjb3BlcyI6WyJhcGlfcmVhZCJdLCJ2ZXJzaW9uIjoxfQ.BkWBY-B7s9Tr6PObyAukp9mC3nerpHeOZcCX9t4BTRE",
	)
	db := db.New(config.DBPath)

	fmt.Printf("starting db...")
	if err := db.Start(); err != nil {
		return err
	}
	fmt.Printf(" ok\n")

	mf := moviefetcher.New(tmdb, db)
	if err := mf.Run(context.Background()); err != nil {
		return err
	}

	return nil
}
