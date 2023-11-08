package libraryserver

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gorm.io/gorm"
	"monks.co/apps/movies/db"
	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
	"monks.co/pkg/tmdb"
)

type LibraryServer struct {
	tmdb  *tmdb.Client
	db    *db.DB
	mutex sync.Mutex
}

type PageData struct {
	Stubs         []*db.Stub
	Movies        []*db.Movie
	Genres        []Genre
	Query         string
	SortBy        string
	SortDirection string
}

type Genre struct {
	Name       string
	IsSelected bool
}

func New(tmdb *tmdb.Client, db *db.DB) *LibraryServer {
	return &LibraryServer{
		tmdb: tmdb,
		db:   db,
	}
}

var port = flag.Int("port", 3001, "port")

func (app *LibraryServer) Run(ctx context.Context) error {
	flag.Parse()

	log.Println("libraryserver started")
	defer log.Println("libraryserver done")

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.serveIndex)
	mux.HandleFunc("/poster", app.servePoster)
	mux.HandleFunc("/play", app.servePlayButton)
	mux.HandleFunc("/search", app.serveSearch)
	mux.HandleFunc("/identify", app.serveIdentify)
	mux.HandleFunc("/ignore", app.serveIgnore)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	s := &http.Server{Addr: addr, Handler: gzip.Middleware(mux)}

	errs := make(chan error)
	go func() {
		if err := s.ListenAndServe(); err != nil {
			log.Println(err)
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

func (app *LibraryServer) serveIndex(w http.ResponseWriter, req *http.Request) {
	log.Println("req /")
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
	if sortBy != "name" && sortBy != "date" && sortBy != "runtime" && sortBy != "importDate" {
		sortBy = "date"
	}
	if sortDirection != "asc" && sortDirection != "desc" {
		if sortBy == "date" || sortBy == "importDate" {
			sortDirection = "desc"
		} else {
			sortDirection = "asc"
		}
	}

	movies, err := app.db.AllMovies()
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	var data PageData
	data.Query = query
	data.SortBy = sortBy
	data.SortDirection = sortDirection

	if stubs, err := app.db.AllStubs(); err != nil {
		serve.InternalServerError(w, req, err)
		return
	} else {
		data.Stubs = stubs
	}

	allGenresSet := map[string]struct{}{}
	for _, movie := range movies {
		if err != nil {
			serve.InternalServerError(w, req, err)
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
			serve.InternalServerError(w, req, err)
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
		case "importDate":
			if data.Movies[a].ImportedAt == "" {
				return true
			} else if data.Movies[b].ImportedAt == "" {
				return true
			}
			if sortDirection == "desc" {
				return data.Movies[a].ImportedAt > data.Movies[b].ImportedAt
			}
			return data.Movies[a].ImportedAt < data.Movies[b].ImportedAt
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

	if err := Page(&data).Render(req.Context(), w); err != nil {
		log.Println(err)
	}
}

func (app *LibraryServer) servePoster(w http.ResponseWriter, req *http.Request) {
	log.Println("req /poster")

	idStr := req.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "error parsing ID: %s", err)
		return
	}
	movie, err := app.db.GetMovie(id)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}
	w.Header().Set("Cache-control", "public, max-age=604800, immutable")
	http.ServeFile(w, req, movie.PosterPath)
}

func (app *LibraryServer) servePlayButton(w http.ResponseWriter, req *http.Request) {
	log.Println("req /play")

	idStr := req.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "error parsing ID: %s", err)
		return
	}
	movie, err := app.db.GetMovie(id)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}
	for _, cmd := range []*exec.Cmd{
		exec.Command("ssh", "lugh", fmt.Sprintf("open -a VLC.app 'sftp://ajm@thor.ss.cx/data/tank/movies/%s'", movie.LibraryPath)),
		exec.Command("ssh", "lugh", `osascript -e 'tell application "VLC" to activate' -e 'tell application "System Events" to keystroke "f" using {command down, control down}'`),
	} {
		cmd := cmd
		if err := cmd.Start(); err != nil {
			serve.InternalServerError(w, req, err)
			return
		}
		go func() {
			if err := cmd.Wait(); err != nil {
				log.Println("start on lugh error:", err)
			}
		}()
	}
	w.WriteHeader(200)
	w.Write([]byte("ok"))
}

func (app *LibraryServer) serveSearch(w http.ResponseWriter, req *http.Request) {
	log.Println("req /search")

	if req.Method != "POST" {
		serve.Errorf(w, req, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	req.ParseForm()

	path := req.FormValue("path")
	query := req.FormValue("query")
	year := req.FormValue("year")

	stub, err := app.db.GetStub(path)
	if err != nil {
		serve.Errorf(w, req, http.StatusNotFound, "no such stub: %s", err)
		return
	}

	log.Println("search", query, year)
	results, err := app.tmdb.Search(query, year)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	stub.Results = results
	log.Printf("%d results", len(results))
	if err := app.db.SaveStub(stub); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	w.WriteHeader(200)
	w.Write([]byte("ok"))
}

func (app *LibraryServer) serveIgnore(w http.ResponseWriter, req *http.Request) {
	log.Println("req /ignore")

	if req.Method != "POST" {
		serve.Errorf(w, req, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	req.ParseForm()

	path := req.FormValue("path")

	stub, err := app.db.GetStub(path)
	if err != nil {
		serve.Errorf(w, req, http.StatusNotFound, "no such stub: %s", err)
		return
	}

	if err := app.db.Transaction(func(tx *gorm.DB) error {
		if err := (&db.DB{DB: tx}).IgnorePath(path); err != nil {
			return err
		}

		if err := (&db.DB{DB: tx}).DeleteStub(stub); err != nil {
			return err
		}
		return nil
	}); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	w.WriteHeader(200)
	w.Write([]byte("ok"))
}
func (app *LibraryServer) serveIdentify(w http.ResponseWriter, req *http.Request) {
	log.Println("req /identify")

	if req.Method != "POST" {
		serve.Errorf(w, req, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	req.ParseForm()

	path := req.FormValue("path")
	id := req.FormValue("id")

	stub, err := app.db.GetStub(path)
	if err != nil {
		serve.Errorf(w, req, http.StatusNotFound, "no such stub: %s", err)
		return
	}

	parsedID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "error parsing ID: %s", err)
		return
	}

	tmdbMovie, err := app.tmdb.Get(parsedID)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	if err := app.db.Transaction(func(tx *gorm.DB) error {
		if _, err := (&db.DB{DB: tx}).CreateMovie(tmdbMovie, path); err != nil {
			return err
		}

		if err := (&db.DB{DB: tx}).DeleteStub(stub); err != nil {
			return err
		}
		return nil
	}); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	w.WriteHeader(200)
	w.Write([]byte("ok"))
}
