package libraryserver

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
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
	mux.HandleFunc("/import", app.serveImport)
	mux.HandleFunc("/poster", app.servePoster)
	mux.HandleFunc("/play", app.servePlayButton)
	mux.HandleFunc("/search", app.serveSearch)
	mux.HandleFunc("/identify", app.serveIdentify)
	mux.HandleFunc("/ignore", app.serveIgnore)
	mux.HandleFunc("/validate-metacritic", app.serveValidateMetacritic)

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
		log.Println("shutting down server")
		s.Shutdown(context.TODO())
		log.Println("done")
	case err := <-errs:
		log.Println("got an err! hopefully we're already shut down.")
		return err
	}

	return nil
}

func (app *LibraryServer) serveIndex(w http.ResponseWriter, req *http.Request) {
	log.Println("serveIndex")
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

	minYear := q.Get("min-year")
	maxYear := q.Get("max-year")

	query := q.Get("query")

	sortBy := q.Get("sort-by")
	sortDirection := q.Get("sort-direction")
	if sortBy != "name" &&
		sortBy != "date" &&
		sortBy != "runtime" &&
		sortBy != "importDate" &&
		sortBy != "mc" &&
		sortBy != "myRating" &&
		sortBy != "watchDate" &&
		sortBy != "shuffle" {
		sortBy = "date"
	}
	if sortDirection != "asc" && sortDirection != "desc" {
		if sortBy == "date" || sortBy == "importDate" || sortBy == "mc" || sortBy == "watchDate" || sortBy == "myRating" {
			sortDirection = "desc"
		} else {
			sortDirection = "asc"
		}
	}

	show := q.Get("show")
	if show != "all" && show != "watched" && show != "unwatched" {
		show = "all"
	}

	movies, err := app.db.AllMovies()
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	watches, err := app.db.AllWatchesMap()
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	var data MoviesPageData
	data.Watches = watches
	data.Query = query
	data.SortBy = sortBy
	data.SortDirection = sortDirection
	data.Show = show
	data.MinYear = minYear
	data.MaxYear = maxYear

	if stubs, err := app.db.AllStubs(); err != nil {
		serve.InternalServerError(w, req, err)
		return
	} else {
		data.Stubs = stubs
	}

	allGenresSet := map[string]struct{}{}
	for _, movie := range movies {
		if sortBy == "watchDate" || sortBy == "myRating" {
			if _, isWatched := watches[movie.Key()]; !isWatched {
				continue
			}
		}

		if sortBy == "myRating" {
			if watch := watches[movie.Key()]; watch.Rating == 0 {
				continue
			}
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

		switch show {
		case "unwatched":
			if _, hasWatch := data.Watches[movie.Key()]; hasWatch {
				continue
			}
		case "watched":
			if _, hasWatch := data.Watches[movie.Key()]; !hasWatch {
				continue
			}
		}

		if !strings.Contains(
			strings.ToLower(movie.Title)+
				" "+strings.ToLower(movie.DirectorName)+
				" "+strings.ToLower(strings.Join(movie.Languages, " "))+
				" "+strings.ToLower(movie.WriterName),
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

	if sortBy == "shuffle" {
		rand.Shuffle(len(data.Movies), func(a, b int) {
			data.Movies[a], data.Movies[b] = data.Movies[b], data.Movies[a]
		})
	} else {
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
			case "watchDate":
				watchA, _ := data.Watches[data.Movies[a].Key()]
				watchB, _ := data.Watches[data.Movies[b].Key()]
				if sortDirection == "desc" {
					return watchA.Date.After(watchB.Date)
				}
				return watchB.Date.After(watchA.Date)
			case "runtime":
				if sortDirection == "desc" {
					return data.Movies[a].Runtime > data.Movies[b].Runtime
				}
				return data.Movies[a].Runtime < data.Movies[b].Runtime
			case "mc":
				if sortDirection == "desc" {
					return data.Movies[a].MetacriticRating > data.Movies[b].MetacriticRating
				}
				return data.Movies[a].MetacriticRating < data.Movies[b].MetacriticRating
			case "myRating":
				watchA, _ := data.Watches[data.Movies[a].Key()]
				watchB, _ := data.Watches[data.Movies[b].Key()]
				if sortDirection == "desc" {
					return watchA.Rating > watchB.Rating
				}
				return watchA.Rating < watchB.Rating
			case "name":
				fallthrough
			default:
				if sortDirection == "desc" {
					return data.Movies[a].Title > data.Movies[b].Title
				}
				return data.Movies[a].Title < data.Movies[b].Title
			}
		})
	}

	if err := Movies(&data).Render(req.Context(), w); err != nil {
		log.Println(err)
	}
}

func (app *LibraryServer) serveImport(w http.ResponseWriter, req *http.Request) {
	log.Println("serveImport")

	var data ImportPageData

	if metacriticValidations, err := app.db.PendingMetacriticValidations(); err != nil {
		serve.InternalServerError(w, req, err)
		return
	} else {
		data.MetacriticValidations = metacriticValidations
	}

	if stubs, err := app.db.AllStubs(); err != nil {
		serve.InternalServerError(w, req, err)
		return
	} else {
		data.Stubs = stubs
	}

	if err := Import(&data).Render(req.Context(), w); err != nil {
		log.Println(err)
	}
}

func (app *LibraryServer) servePoster(w http.ResponseWriter, req *http.Request) {
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

	app.serveIndex(w, req)
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

	if err := app.db.Transaction(func(tx *db.DB) error {
		if err := tx.IgnorePath(stub.Type, path); err != nil {
			return err
		}

		if err := tx.DeleteStub(stub); err != nil {
			return err
		}
		return nil
	}); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	app.serveIndex(w, req)
}

func (app *LibraryServer) serveIdentify(w http.ResponseWriter, req *http.Request) {
	log.Println("req /identify")

	if req.Method != "POST" {
		serve.Errorf(w, req, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	req.ParseForm()

	path := req.FormValue("path")
	if path == "" {
		serve.Errorf(w, req, http.StatusBadRequest, "no path given")
		return
	}

	stub, err := app.db.GetStub(path)
	if err != nil {
		serve.Errorf(w, req, http.StatusNotFound, "no such stub: %s", err)
		return
	}

	id := req.FormValue("id")
	parsedID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "error parsing ID: %s", err)
		return
	}

	tmdbMovie, err := app.tmdb.Get(parsedID)
	if err != nil {
		serve.InternalServerErrorf(w, req, "error getting movie metadata from tmdb %w", err)
		return
	}

	if err := app.db.Transaction(func(tx *db.DB) error {
		movie, err := tx.GetMovie(tmdbMovie.ID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("error getting movie with id %d: %w", tmdbMovie.ID, err)
		}

		if movie != nil {
			// movie already exists; replace
			if err := tx.ReplaceMovie(movie, path); err != nil {
				return fmt.Errorf("error replacing movie: %w (tmdb id %d)", err, tmdbMovie.ID)
			}
		} else {
			// new movie; create
			fmt.Println("create", movie)
			if _, err := tx.CreateMovie(tmdbMovie, path); err != nil {
				return fmt.Errorf("error creating movie: %w", err)
			}
		}

		if err := tx.DeleteStub(stub); err != nil {
			return fmt.Errorf("error deleting movie: %w", err)
		}
		return nil
	}); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	app.serveImport(w, req)
}

func (app *LibraryServer) serveValidateMetacritic(w http.ResponseWriter, req *http.Request) {
	log.Println("req /validate-metacritic")

	if req.Method != "POST" {
		serve.Errorf(w, req, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	req.ParseForm()

	idStr := req.FormValue("Movie ID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}
	url := req.FormValue("Metacritic URL")
	ratingStr := req.FormValue("Rating")
	rating, err := strconv.ParseInt(ratingStr, 10, 64)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	movie, err := app.db.GetMovie(id)
	if err != nil {
		serve.Errorf(w, req, http.StatusNotFound, "no such movie: %s", err)
		return
	}

	fmt.Println(movie.Title, url, id, rating)

	if err := app.db.Transaction(func(tx *db.DB) error {
		if err := tx.ValidateMovieMetacriticData(movie, url, int(rating)); err != nil {
			return err
		}
		if err := tx.AddMovieRating(movie, int(rating), url); err != nil {
			return err
		}
		return nil
	}); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	fmt.Println("written", movie.ID)

	app.serveImport(w, req)
}
