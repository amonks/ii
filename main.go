package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"monks.co/movietagger/db"
	"monks.co/movietagger/fzf"
	"monks.co/movietagger/tmdb"
)

type App struct {
	db   *db.DB
	tmdb *tmdb.Client
}

func main() {
	logfile, err := os.OpenFile("movietagger.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(logfile)
	defer logfile.Close()

	app := &App{
		db:   db.New("/mypool/tank/movies/.movies.db"),
		tmdb: tmdb.New("88f973483e2dc73cfb5053bc059ae33b"),
	}

	if err := app.db.Migrate(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	conn := app.db.Connect()
	defer app.db.Put(conn)

	if err := filepath.Walk("/mypool/data/mirror/whatbox/files/movies", func(path string, info os.FileInfo, err error) error {
		if !info.Mode().IsRegular() {
			return nil
		}
		if exists, err := app.db.MovieExistsFromPath(conn, path); err != nil {
			return err
		} else if exists {
			return nil
		}

		if !strings.HasSuffix(path, "mkv") {
			log.Println("skip", path)
			return nil
		}

		search:
		titleQ, yearQ := app.buildSearchQuery(path)
		if titleQ == "" {
			return nil
		}

		tmdbMovie, err := app.search(titleQ, yearQ)
		if err == errRetry {
			goto search
		} else if err == errSkip {
			return nil
		} else if err != nil {
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
	}); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
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
		if err == nil {
			year = int64(i)
		}
	}

	return titleQ, year
}

var errRetry = errors.New("retry")
var errSkip = errors.New("skip")

func (a *App) search(titleQ string, yearQ int64) (*tmdb.Movie, error) {
	ress, err := a.tmdb.Search(titleQ, yearQ)
	if err != nil {
		return nil, err
	}

	fzfTerms := []string{"retry", "skip"}
	idsByTerm := make(map[string]int64)
	for _, res := range ress {
		term := fmt.Sprintf("%s -- %s", res.ReleaseDate, res.Title)
		fzfTerms = append(fzfTerms, term)
		idsByTerm[term] = res.ID
	}

	term, err := fzf.Select(fzfTerms)
	if err != nil {
		return nil, err
	}

	if term == "retry" {
		return nil, errRetry
	}
	if term == "skip" {
		return nil, errSkip
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
