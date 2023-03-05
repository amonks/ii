package movietagger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"monks.co/movietagger/db"
	"monks.co/movietagger/system"
	"monks.co/movietagger/tmdb"
	"monks.co/movietagger/ui"
)

type MovieTagger struct {
	*system.System
	tmdb *tmdb.Client
	db   *db.DB
}

func New(tmdb *tmdb.Client, db *db.DB) *MovieTagger {
	system := system.New("tagger")
	return &MovieTagger{
		System: system,
		tmdb:   tmdb,
		db:     db,
	}
}

func (app *MovieTagger) Run() error {
	defer app.System.Start()()

	fmt.Println("sanitizing filenames")
	if err := app.sanitizeFilenames(); err != nil {
		return err
	}
	fmt.Println("done sanitizing filenames")

	if err := filepath.Walk("/mypool/data/mirror/whatbox/files/movies", func(path string, info os.FileInfo, err error) error {
		if !info.Mode().IsRegular() {
			return nil
		}

		if !strings.HasSuffix(path, "mkv") {
			app.Printf("skip %s", path)
			return nil
		}

		if ignored, err := app.db.PathIsIgnored(path); err != nil {
			return err
		} else if ignored {
			app.Printf("ignore %s", path)
			return nil
		}

		if exists, err := app.db.MovieExistsFromPath(path); err != nil {
			return err
		} else if exists {
			app.Printf("duplicate %s", path)
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
			if err := app.db.IgnorePath(path); err != nil {
				return err
			}
			return nil
		}

		fmt.Printf("matching %d %s...\n", yearQ, titleQ)
		tmdbID, err := app.search(titleQ, yearQ)
		if err == errRetry {
			goto search
		} else if err == errSkip {
			fmt.Printf("skipping.\n")
			if err := app.db.IgnorePath(path); err != nil {
				return err
			}
			return nil
		} else if err != nil {
			return err
		}

		fmt.Printf("looking up movie metadata...")
		tmdbMovie, err := app.tmdb.Get(tmdbID)
		if err != nil {
			fmt.Printf("\n")
			return err
		}
		fmt.Printf(" OK\n")

		fmt.Printf("adding movie to database...")
		movie := db.NewMovie(tmdbMovie, path)
		if err := app.db.AddMovie(movie); errors.Is(err, db.ErrCollision) {
			existing, err := app.db.GetMovie(movie.ID)
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
				if err := app.db.ReplaceMovie(movie.ID, movie.ImportedFromPath); err != nil {
					return err
				}
				return nil
			case "no":
				fmt.Println("ok. skipping.")
				if err := app.db.IgnorePath(path); err != nil {
					return err
				}
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

func (a *MovieTagger) sanitizeFilenames() error {
	ids, err := a.db.AllMovies()
	if err != nil {
		return err
	}

	for _, id := range ids {
		movie, err := a.db.GetMovie(id)
		if err != nil {
			return err
		}

		correctPath := movie.BuildLibraryPath()
		if movie.LibraryPath != correctPath {
			fmt.Printf("'%s' should be '%s'\n", movie.LibraryPath, correctPath)
		prompt:
			response := ui.Prompt("correct? [yes,no]")
			switch response {
			case "yes":
				if err := a.db.RebuildLibraryPath(movie); err != nil {
					return err
				}
				continue
			case "no":
				continue
			default:
				goto prompt
			}
		}
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
	ress, err := a.tmdb.Search(titleQ, yearQ)
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
