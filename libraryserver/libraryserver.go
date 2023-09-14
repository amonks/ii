package libraryserver

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"os/exec"
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

		selectedGenres := q["genres"]
		selectedGenresSet := map[string]struct{}{}
		for _, g := range selectedGenres {
			selectedGenresSet[g] = struct{}{}
		}
		allGenresSelected := false
		if len(selectedGenres) == 0 {
			allGenresSelected = true
		}

		minYear := q.Get("minYear")
		maxYear := q.Get("maxYear")

		query := q.Get("query")

		sortBy := q.Get("sortBy")
		sortDirection := q.Get("sortDirection")
		if sortBy != "name" && sortBy != "date" && sortBy != "runtime" {
			sortBy = "date"
		}
		if sortDirection != "asc" && sortDirection != "desc" {
			if sortBy == "date" {
				sortDirection = "desc"
			} else {
				sortDirection = "asc"
			}
		}

		ids, err := app.db.AllMovies()
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(500)
			w.Write([]byte("error"))
			return
		}

		type Genre struct {
			Name       string
			IsSelected bool
		}
		var data struct {
			Movies        []*db.Movie
			Genres        []Genre
			Query         string
			SortBy        string
			SortDirection string
		}
		data.Query = query
		data.SortBy = sortBy
		data.SortDirection = sortDirection

		allGenresSet := map[string]struct{}{}
		for _, id := range ids {
			movie, err := app.db.GetMovie(id)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(500)
				w.Write([]byte("error"))
				return
			}

			genreMatch := false
			for _, g := range movie.Genres {
				if len(g) == 0 {
					continue
				}
				allGenresSet[g] = struct{}{}
				for _, gg := range selectedGenres {
					if gg == g {
						genreMatch = true
					}
				}
			}
			if !genreMatch && !allGenresSelected {
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

			if !strings.Contains(
				strings.ToLower(movie.Title)+" "+strings.ToLower(movie.DirectorName)+" "+strings.ToLower(movie.WriterName),
				strings.ToLower(query)) {
				continue
			}

			data.Movies = append(data.Movies, movie)
		}

		for genre := range allGenresSet {
			_, isSelected := selectedGenresSet[genre]
			data.Genres = append(data.Genres, Genre{
				Name:       genre,
				IsSelected: !allGenresSelected && isSelected,
			})
		}
		sort.Slice(data.Genres, func(a, b int) bool {
			return data.Genres[a].Name < data.Genres[b].Name
		})

		sort.Slice(data.Movies, func(a, b int) bool {
			switch sortBy {
			case "date":
				if sortDirection == "desc" {
					return data.Movies[a].ReleaseDate > data.Movies[b].ReleaseDate
				}
				return data.Movies[a].ReleaseDate < data.Movies[b].ReleaseDate
			case "runtime":
				if sortDirection == "desc" {
					return data.Movies[a].Runtime > data.Movies[b].Runtime
				}
				return data.Movies[a].Runtime < data.Movies[b].Runtime
			case "name":
				fallthrough
			default:
				if sortDirection == "desc" {
					return data.Movies[a].Title > data.Movies[b].Title
				}
				return data.Movies[a].Title < data.Movies[b].Title
			}
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
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("error"))
			return
		}
		w.Header().Set("Cache-control", "public, max-age=604800, immutable")
		http.ServeFile(w, req, movie.PosterPath)
	})
	mux.HandleFunc("/play", func(w http.ResponseWriter, req *http.Request) {
		idStr := req.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("error"))
			return
		}
		movie, err := app.db.GetMovie(id)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("error"))
			return
		}
		for _, cmd := range []*exec.Cmd{
			exec.Command("ssh", "lugh", fmt.Sprintf("open -a VLC.app 'sftp://ajm@thor.ss.cx/mypool/tank/movies/%s'", movie.LibraryPath)),
			exec.Command("ssh", "lugh", `osascript -e 'tell application "VLC" to activate' -e 'tell application "System Events" to keystroke "f" using {command down, control down}'`),
		} {
			cmd := cmd
			if err := cmd.Start(); err != nil {
				w.WriteHeader(500)
				w.Write([]byte("error"))
				return
			}
			go func() {
				if err := cmd.Wait(); err != nil {
					fmt.Println("MOVIE ERROR")
					fmt.Println(err)
				}
			}()
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	wrapped := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fmt.Println(req.URL.String())
		mux.ServeHTTP(w, req)
	})

	s := &http.Server{Addr: "0.0.0.0:3333", Handler: wrapped}

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
