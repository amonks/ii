package libraryserver

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"monks.co/movietagger/db"
	"monks.co/movietagger/system"
	"monks.co/movietagger/tmdb"
)

var (
	//go:embed templates/movies.gohtml
	tmplSrc string
)

type LibraryServer struct {
	*system.System
	tmdb  *tmdb.Client
	db    *db.DB
	mutex sync.Mutex
}

func New(tmdb *tmdb.Client, db *db.DB) *LibraryServer {
	system := system.New("libraryserver")
	return &LibraryServer{
		System: system,
		tmdb:   tmdb,
		db:     db,
	}
}

func (app *LibraryServer) Run(ctx context.Context) error {
	defer app.System.Start()()

	fmt.Println("libraryserver: start")

	tmpl := template.New("movies")
	tmpl, err := tmpl.Parse(tmplSrc)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		q := req.URL.Query()
		genres := q["genres"]
		minYear := q.Get("minYear")
		maxYear := q.Get("maxYear")
		query := q.Get("query")

		ids, err := app.db.AllMovies()
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(500)
			w.Write([]byte("error"))
			return
		}

		var data struct{ Movies []*db.Movie }
		for _, id := range ids {
			movie, err := app.db.GetMovie(id)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
				w.Write([]byte("error"))
				return
			}
			genreMatch := true
			if len(genres) > 0 {
				genreMatch = false
				for _, g := range movie.Genres {
					for _, gg := range genres {
						if g == gg {
							genreMatch = true
						}
					}
				}
			}
			if !genreMatch {
				continue
			}

			year, err := strconv.ParseInt(movie.ReleaseDate[0:4], 10, 64)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
				w.Write([]byte("error"))
				return
			}
			if minYear, err := strconv.ParseInt(minYear, 10, 64); err == nil && year < minYear {
				continue
			}
			if maxYear, err := strconv.ParseInt(maxYear, 10, 64); err == nil && year > maxYear {
				continue
			}

			if !strings.Contains(movie.Title, query) {
				continue
			}

			data.Movies = append(data.Movies, movie)
		}

		sort.Slice(data.Movies, func(a, b int) bool {
			return data.Movies[a].ReleaseDate < data.Movies[b].ReleaseDate
		})

		if err := tmpl.Execute(w, data); err != nil {
			fmt.Println(err)
		}
	})
	mux.HandleFunc("/poster", func(w http.ResponseWriter, req *http.Request) {
		idStr := req.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("error"))
			return
		}
		movie, err := app.db.GetMovie(id)
		w.Header().Set("Cache-control", "public, max-age=604800, immutable")
		http.ServeFile(w, req, movie.PosterPath)
	})

	s := &http.Server{Addr: "0.0.0.0:3333", Handler: mux}

	errs := make(chan error)
	go func() {
		if err := s.ListenAndServe(); err != nil {
			fmt.Println(err)
			errs <- err
		}
	}()

	select {
	case <-ctx.Done():
		s.Shutdown(context.TODO())
	case err := <-errs:
		return err
	}

	return nil
}
