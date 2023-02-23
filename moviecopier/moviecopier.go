package main

import (
	"fmt"
	"log"
	"os"

	"monks.co/movietagger/db"
	"monks.co/movietagger/system"
	"monks.co/movietagger/tmdb"
)

func main() {
	logfile, err := os.OpenFile("moviecopier.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
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

	for {
		fmt.Printf("getting next movie...")
		nextMovie, err := app.DB.GetMovieToImport(conn)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf(" OK: %s\n", nextMovie.Title)

		fmt.Printf("copying movie...")
		if err := app.CopyFile(nextMovie.ImportedFromPath, nextMovie.LibraryPath); err != nil {
			log.Fatal(err)
		}
		fmt.Printf(" OK\n")

		fmt.Printf("marking as imported...")
		if err := app.DB.MarkMovieAsImported(conn, nextMovie.ID); err != nil {
			log.Fatal(err)
		}
		fmt.Printf(" OK\n")
	}
}
