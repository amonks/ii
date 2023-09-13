package db

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"monks.co/movietagger/tmdb"
)

type Movie struct {
	ID int64

	Title         string
	OriginalTitle string
	Tagline       string
	Overview      string
	Runtime       int64
	Genres        []string
	Languages     []string
	ReleaseDate   string

	Extension        string
	LibraryPath      string
	ImportedFromPath string

	IsImported bool

	TMDBJSON   string
	PosterPath string
}

func NewMovie(m *tmdb.Movie, importedFromPath string) *Movie {
	var genres []string
	var languages []string
	for _, genre := range m.Genres {
		genres = append(genres, genre.Name)
	}
	for _, language := range m.Languages {
		languages = append(languages, language.Name)
	}
	movie := Movie{
		ID:            m.ID,
		Title:         m.Title,
		OriginalTitle: m.OriginalTitle,
		Tagline:       m.Tagline,
		Overview:      m.Overview,
		Runtime:       m.RunTime,
		Genres:        genres,
		Languages:     languages,
		ReleaseDate:   m.ReleaseDate,

		Extension:        filepath.Ext(importedFromPath),
		ImportedFromPath: importedFromPath,

		IsImported: false,

		TMDBJSON: m.TMDBJSON,
	}
	movie.LibraryPath = movie.BuildLibraryPath()
	return &movie
}

var illegalCharForFilename = regexp.MustCompile(`\/`)

func (m *Movie) BuildLibraryPath() string {
	releaseYear := m.ReleaseDate[0:4]
	filename := fmt.Sprintf("%s-%s%s", releaseYear, m.Title, m.Extension)
	filename = illegalCharForFilename.ReplaceAllString(filename, "-")
	return filename
}

func (d *DB) AddMovie(movie *Movie) error {
	c, err := d.conn()
	defer c.release()
	if err != nil {
		return err
	}

	const q = `insert into movies (id, title, original_title, tagline, overview, runtime, genres, languages, release_date, extension, library_path, imported_from_path, is_imported, tmdb_json)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	if err := sqlitex.Exec(c.Conn, q, nil,
		movie.ID, movie.Title, movie.OriginalTitle, movie.Tagline, movie.Overview, movie.Runtime, join(movie.Genres), join(movie.Languages), movie.ReleaseDate, movie.Extension, movie.LibraryPath, movie.ImportedFromPath, false, movie.TMDBJSON,
	); err != nil {
		sqliteError, isSqliteError := err.(sqlite.Error)
		if isSqliteError && sqliteError.Code == sqlite.SQLITE_CONSTRAINT_UNIQUE {
			err = errors.Join(err, ErrCollision)
		}
		return fmt.Errorf("failed to insert movie: %w", err)
	}

	return nil
}

func (d *DB) AddMovieJSON(id int64, json string) error {
	c, err := d.conn()
	defer c.release()
	if err != nil {
		return err
	}

	const q = `update movies set tmdb_json = ? where id = ?;`
	if err := sqlitex.Exec(c.Conn, q, nil, json, id); err != nil {
		return fmt.Errorf("failed to set movie json: %w", err)
	}

	return nil
}

func (d *DB) AddMoviePoster(id int64, posterPath string) error {
	c, err := d.conn()
	defer c.release()
	if err != nil {
		return err
	}

	const q = `update movies set poster_path = ? where id = ?;`
	if err := sqlitex.Exec(c.Conn, q, nil, posterPath, id); err != nil {
		return fmt.Errorf("failed to set movie poster: %w", err)
	}

	return nil
}

func (d *DB) GetMovieIDToImport() (int64, error) {
	c, err := d.conn()
	defer c.release()
	if err != nil {
		return 0, err
	}

	const q = `select id from movies where is_imported = false limit 1;`
	var id int64
	onResult := func(stmt *sqlite.Stmt) error {
		id = stmt.ColumnInt64(0)
		return nil
	}
	if err := sqlitex.Exec(c.Conn, q, onResult); err != nil {
		return 0, fmt.Errorf("failed to fetch next movie to import: %w", err)
	}

	return id, nil
}

func (d *DB) MarkMovieAsImported(id int64) error {
	c, err := d.conn()
	defer c.release()
	if err != nil {
		return err
	}

	const q = `update movies set is_imported = true where id = ?;`
	if err := sqlitex.Exec(c.Conn, q, nil, id); err != nil {
		return fmt.Errorf("failed to mark movie as imported: %w", err)
	}

	return nil
}

func (d *DB) ReplaceMovie(id int64, path string) error {
	c, err := d.conn()
	defer c.release()
	if err != nil {
		return err
	}

	const q = `update movies set is_imported = false, imported_from_path = ? where id = ?;`
	if err := sqlitex.Exec(c.Conn, q, nil, path, id); err != nil {
		return fmt.Errorf("failed to replace movie: %w", err)
	}

	return nil
}

func (d *DB) DeleteMovie(id int64) error {
	c, err := d.conn()
	defer c.release()
	if err != nil {
		return err
	}

	const q = `delete from movies where id = ?`
	if err := sqlitex.Exec(c.Conn, q, nil, id); err != nil {
		return fmt.Errorf("failed to delete movie: %w", err)
	}

	return nil
}

func (d *DB) MovieExistsFromPath(importedFromPath string) (bool, error) {
	c, err := d.conn()
	defer c.release()
	if err != nil {
		return false, err
	}

	const q = `select true from movies where imported_from_path = ?;`
	wasImported := false
	f := func(stmt *sqlite.Stmt) error {
		wasImported = true
		return nil
	}
	if err := sqlitex.Exec(c.Conn, q, f, importedFromPath); err != nil {
		return wasImported, fmt.Errorf("failed to check for movie at path '%s': %w", importedFromPath, err)
	}
	return wasImported, nil
}

func (d *DB) AllMovies() ([]int64, error) {
	c, err := d.conn()
	defer c.release()
	if err != nil {
		return nil, err
	}

	const q = `select id from movies;`
	var ids []int64
	f := func(stmt *sqlite.Stmt) error {
		ids = append(ids, stmt.ColumnInt64(0))
		return nil
	}
	if err := sqlitex.Exec(c.Conn, q, f); err != nil {
		return nil, err
	}

	return ids, nil
}

func (m *Movie) PosterURL() string {
	return fmt.Sprintf("/poster?id=%d", m.ID)
}

func (d *DB) GetMovie(id int64) (*Movie, error) {
	c, err := d.conn()
	defer c.release()
	if err != nil {
		return nil, err
	}

	const q = `select id, title, original_title, tagline, overview, runtime, genres, languages, release_date, extension, library_path, imported_from_path, is_imported, tmdb_json, poster_path from movies where id = ?;`
	var movie Movie
	f := func(stmt *sqlite.Stmt) error {
		var json []byte
		stmt.ColumnBytes(4, json)
		movie = Movie{
			ID:            stmt.ColumnInt64(0),
			Title:         stmt.ColumnText(1),
			OriginalTitle: stmt.ColumnText(2),
			Tagline:       stmt.ColumnText(3),
			Overview:      stmt.ColumnText(4),
			Runtime:       stmt.ColumnInt64(5),
			Genres:        split(stmt.ColumnText(6)),
			Languages:     split(stmt.ColumnText(7)),
			ReleaseDate:   stmt.ColumnText(8),

			Extension:        stmt.ColumnText(9),
			LibraryPath:      stmt.ColumnText(10),
			ImportedFromPath: stmt.ColumnText(11),
			IsImported:       stmt.ColumnInt(12) == 1,
			TMDBJSON:         stmt.ColumnText(13),
			PosterPath:       stmt.ColumnText(14),
		}
		return nil
	}
	if err := sqlitex.Exec(c.Conn, q, f, id); err != nil {
		return nil, fmt.Errorf("failed to get movie: %w", err)
	}
	return &movie, nil
}
