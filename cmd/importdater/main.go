package main

import (
	"fmt"
	"os"
	"path/filepath"

	"monks.co/movietagger/config"
	"monks.co/movietagger/db"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	db := db.New(config.DBPath)
	if err := db.Start(); err != nil {
		return err
	}

	movies, err := db.AllMovies()
	if err != nil {
		return err
	}
	for _, movie := range movies {
		stat, err := os.Stat(filepath.Join(config.MovieLibraryDir, movie.LibraryPath))
		if err != nil {
			return err
		}
		fmt.Println(stat.ModTime(), movie.Title)
		if err := db.SetMovieImportedAt(movie, stat.ModTime()); err != nil {
			return err
		}
	}

	return nil
}
