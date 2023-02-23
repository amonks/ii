package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"monks.co/movietagger/db"
	"monks.co/movietagger/system"
	"monks.co/movietagger/tmdb"
)

func main() {
	logfile, err := os.OpenFile("movietagger.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(logfile)
	defer logfile.Close()

	app := &system.System{
		DB:   db.New("/mypool/tank/movies/.movies.db"),
		TMDB: tmdb.New("88f973483e2dc73cfb5053bc059ae33b"),
	}

	if err := app.DB.Migrate(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	conn := app.DB.Connect()
	defer app.DB.Put(conn)

	if err := filepath.Walk("/mypool/data/mirror/whatbox/files/movies", func(path string, info os.FileInfo, err error) error {
		if !info.Mode().IsRegular() {
			return nil
		}
		if exists, err := app.DB.MovieExistsFromPath(conn, path); err != nil {
			return err
		} else if exists {
			return nil
		}

		if !strings.HasSuffix(path, "mkv") {
			log.Println("skip", path)
			return nil
		}

	search:
		titleQ, yearQ, err := app.BuildSearchQuery(path)
		if err != nil {
			fmt.Printf("error with search: %s\n", err)
			goto search
		}
		if titleQ == "" {
			fmt.Printf("skipping.\n")
			return nil
		}

		fmt.Printf("matching %d %s...\n", yearQ, titleQ)
		tmdbID, err := app.Search(titleQ, yearQ)
		if err == system.ErrRetry {
			goto search
		} else if err == system.ErrSkip {
			fmt.Printf("Skipping.\n")
			return nil
		} else if err != nil {
			return err
		}

		fmt.Printf("looking up movie metadata...")
		tmdbMovie, err := app.TMDB.Get(tmdbID)
		if err != nil {
			fmt.Printf("\n")
			return err
		}
		fmt.Printf(" OK\n")

		fmt.Printf("adding movie to database...")
		movie := db.NewMovie(tmdbMovie, path)
		if err := app.DB.AddMovie(conn, movie); errors.Is(err, db.ErrCollision) {
			fmt.Printf("\ncollision. retrying...\n")
			goto search
		} else if err != nil {
			return err
		}
		fmt.Printf(" OK\n")

		// fmt.Printf("copying movie to library...")
		// if err := copyFile(path, movie.LibraryPath); err != nil {
		// 	return err
		// }
		// fmt.Printf(" OK\n")

		fmt.Printf("Done\n")
		return nil
	}); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

