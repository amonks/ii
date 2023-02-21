package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"crawshaw.io/sqlite"
	"monks.co/movietagger/db"
	"monks.co/movietagger/fzf"
	"monks.co/movietagger/tmdb"
)

type App struct {
	db   *db.DB
	tmdb *tmdb.Client
}

func main() {
	app := &App{
		db:   db.New("data/movies.db"),
		tmdb: tmdb.New("88f973483e2dc73cfb5053bc059ae33b"),
	}

	if err := app.db.Migrate(); err != nil {
		log.Fatal(err)
	}

	conn := app.db.Connect()
	defer app.db.Put(conn)

	filepath.Walk("/mypool/data/mirror/whatbox/files/movies", func(path string, info os.FileInfo, err error) error {
		if !info.Mode().IsRegular() {
			return nil
		}
		if exists, err := app.db.MovieExistsFromPath(conn, path); err != nil {
			return err
		} else if exists {
			return nil
		}

		titleQ, yearQ := app.buildSearchQuery(path)
		if titleQ == "" {
			return nil
		}

		tmdbMovie, err := app.search(titleQ, yearQ)
		if err != nil {
			return err
		}

		movie := db.NewMovie(tmdbMovie, path)

		if err := app.db.AddMovie(conn, movie); err != nil {
			return err
		}

		if err := copyFile(path, movie.LibraryPath); err != nil {
			return err
		}

		return nil
	})

	results, err := app.tmdb.Search("inception", 0)
	if err != nil {
		log.Fatal(err)
	}

	movie, err := app.tmdb.Get(results[0].ID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(movie.ReleaseDate, movie.Title, movie.Overview)
}

func (a *App) buildSearchQuery(path string) (string, int64) {
	fmt.Println("locating " + path)

	var yearQ string
	var titleQ string

	fmt.Println("enter year")
	fmt.Scanln(&yearQ)

	fmt.Println("enter query")
	fmt.Scanln(&titleQ)

	var year int64
	if yearQ != "" {
		i, err := strconv.Atoi(yearQ)
		if err != nil {
			year = int64(i)
		}
	}

	return titleQ, year
}

func (a *App) search(titleQ string, yearQ int64) (*tmdb.Movie, error) {
	ress, err := a.tmdb.Search(titleQ, yearQ)
	if err != nil {
		return nil, err
	}

	var fzfTerms []string
	var idsByTerm map[string]int64
	for _, res := range ress {
		term := fmt.Sprintf("%s -- %s", res.ReleaseDate, res.Title)
		fzfTerms = append(fzfTerms, term)
		idsByTerm[term] = res.ID
	}

	term, err := fzf.Select(fzfTerms)
	if err != nil {
		return nil, err
	}

	id := idsByTerm[term]

	movie, err := a.tmdb.Get(id)
	if err != nil {
		return nil, err
	}

	return movie, nil
}

func copyFile(src, dest string) error {
	srcStat, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcStat.Mode().IsRegular() {
		return fmt.Errorf("cannot copy irregular file '%s'", src)
	}

	srcF, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening file '%s': %w", src, err)
	}
	defer srcF.Close()

	destF, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("error creating file '%s': %w", dest, err)
	}

	if _, err := io.Copy(destF, srcF); err != nil {
		return fmt.Errorf("error copying file from '%s' to '%s': %w", src, dest, err)
	}

	return nil
}
