package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
	"monks.co/pkg/tui"
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

	movies, err := db.AllMovies()
	if err != nil {
		return err
	}

	for _, movie := range movies {
		if movie.MetacriticValidated {
			continue
		}

		fmt.Println("-----------")
		fmt.Println(movie.ReleaseDate, movie.Title)
		fmt.Println(movie.MetacriticURL, movie.MetacriticRating)

		// If it has a rating, validate it.
		if movie.MetacriticRating != 0 {
			// If valid, mark as valid and continue.
			if ok := tui.Prompt("ok? y/n"); ok == "y" {
				if err := db.SetMovieMetacriticValidated(movie, true); err != nil {
					return err
				}
				continue
			}
		}

		// Input path.

	input:
		url := tui.Prompt("url")
		scoreStr := tui.Prompt("score")
		score, err := strconv.ParseInt(scoreStr, 10, 64)
		if err != nil {
			fmt.Println("parse fail")
			goto input
		}

		if err := db.AddMovieRating(movie, int(score), url); err != nil {
			return err
		}
		if err := db.SetMovieMetacriticValidated(movie, true); err != nil {
			return err
		}
	}

	return nil
}
