package movietagger

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"monks.co/movietagger/db"
	"monks.co/movietagger/system"
	"monks.co/movietagger/ui"
)

type MovieTagger struct {
	system.System
}

func New(system system.System) *MovieTagger {
	return &MovieTagger{system}
}

func (app *MovieTagger) Run() error {
	logfile, err := os.OpenFile("movietagger.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		os.Exit(1)
	}
	defer logfile.Close()
	logger := log.New(logfile, "", log.Ldate | log.Ltime)

	if err := filepath.Walk("/mypool/data/mirror/whatbox/files/movies", func(path string, info os.FileInfo, err error) error {
		if !info.Mode().IsRegular() {
			return nil
		}

		if !strings.HasSuffix(path, "mkv") {
			logger.Println("skip", path)
			return nil
		}

		if ignored, err := app.DB.PathIsIgnored(path); err != nil {
			return err
		} else if ignored {
			logger.Println("ignore", path)
			return nil
		}

		if exists, err := app.DB.MovieExistsFromPath(path); err != nil {
			return err
		} else if exists {
			logger.Println("duplicate", path)
			return nil
		}

	search:
		titleQ, yearQ, err := app.buildSearchQuery(path)
		if err != nil {
			fmt.Printf("error with search: %s\n", err)
			goto search
		}
		if titleQ == "" {
			fmt.Printf("skipping.\n")
			return nil
		}

		fmt.Printf("matching %d %s...\n", yearQ, titleQ)
		tmdbID, err := app.search(titleQ, yearQ)
		if err == errRetry {
			goto search
		} else if err == errSkip {
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
		if err := app.DB.AddMovie(movie); errors.Is(err, db.ErrCollision) {
			existing, err := app.DB.Get(movie.ID)
			if err != nil {
				fmt.Printf("\n")
				return err
			}

		collision:
			fmt.Printf("collision\n")
			fmt.Println(existing.ImportedFromPath)
			response := ui.Prompt("overwrite? [yes,no,retry]")
			if err != nil {
				return err
			}

			switch response {
			case "yes":
				fmt.Println("ok. overwriting.")
				if err := app.DB.ReplaceMovie(movie.ID, movie.ImportedFromPath); err != nil {
					return err
				}
				return nil
			case "no":
				fmt.Println("ok. skipping.")
				return nil
			case "retry":
				goto search
			default:
				fmt.Println("bad response.")
				goto collision
			}
		} else if err != nil {
			return err
		} else {
			fmt.Printf(" OK\n")
		}

		fmt.Printf("Done\n")
		return nil
	}); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return nil
}

func (a *MovieTagger) buildSearchQuery(path string) (string, int64, error) {
	fmt.Println("locating " + path)

	yearQ := ui.Prompt("year")
	titleQ := ui.Prompt("query")

	var year int64
	if yearQ != "" {
		i, err := strconv.Atoi(yearQ)
		if err == nil {
			year = int64(i)
		}
	}

	return titleQ, year, nil
}

var errRetry = errors.New("retry")
var errSkip = errors.New("skip")

func (a *MovieTagger) search(titleQ string, yearQ int64) (int64, error) {
	ress, err := a.TMDB.Search(titleQ, yearQ)
	if err != nil {
		return 0, err
	}

	fzfTerms := []string{}
	idsByTerm := make(map[string]int64)
	for _, res := range ress {
		tmdbURL := fmt.Sprintf("https://www.themoviedb.org/movie/%d", res.ID)
		term := fmt.Sprintf("%s: %s %s", res.ReleaseDate, res.Title, tmdbURL)
		fzfTerms = append(fzfTerms, term)
		idsByTerm[term] = res.ID
	}
	fzfTerms = append(fzfTerms, "retry", "skip", "manual entry")

	term, err := ui.Select("which?", fzfTerms)
	if err != nil {
		return 0, err
	}

	if term == "manual entry" {
		var idQ string
		fmt.Printf("enter ID: ")
		if _, err := fmt.Scanln(&idQ); err != nil {
			return 0, err
		}
		id, err := strconv.Atoi(idQ)
		if err != nil {
			return 0, err
		}
		return int64(id), nil
	}

	if term == "retry" {
		return 0, errRetry
	}
	if term == "skip" {
		return 0, errSkip
	}

	id := idsByTerm[term]

	return id, nil
}
