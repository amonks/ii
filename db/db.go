package db

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"monks.co/movietagger/tmdb"
)

type DB struct {
	path string
	pool *sqlitex.Pool
}

func New(path string) *DB {
	return &DB{
		path: path,
	}
}

func (db *DB) Connect() *sqlite.Conn {
	return db.pool.Get(context.Background())
}

func (db *DB) Put(conn *sqlite.Conn) {
	db.pool.Put(conn)
}

func (db *DB) Migrate() error {
	pool, err := sqlitex.Open(fmt.Sprintf("file:%s", db.path), 0, 10)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	db.pool = pool

	conn := db.pool.Get(context.Background())
	if conn == nil {
		return fmt.Errorf("failed to get db connection")
	}
	defer db.pool.Put(conn)

	if err := sqlitex.Exec(conn, `PRAGMA journal_mode=wal;`, nil); err != nil {
		return fmt.Errorf("failed to enable wal mode: %w", err)
	}

	if err := sqlitex.Exec(conn, `
		create table if not exists movies (
			id                 int primary key,
			title              text,
			original_title     text,
			tagline            text,
			overview           text,
			runtime            int,
			genres             text,
			languages          text,
			release_date       text,

			extension          text,
			library_path       text unique,
			imported_from_path text unique,

			is_imported        integer
		);`, nil); err != nil {
		return fmt.Errorf("failed to create `movies` table: %w", err)
	}

	return nil
}

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
	}
	movie.LibraryPath = movie.BuildLibraryPath()
	return &movie
}

func (m *Movie) BuildLibraryPath() string {
	releaseYear := m.ReleaseDate[0:4]
	return fmt.Sprintf("/mypool/tank/movies/%s-%s%s", releaseYear, m.Title, m.Extension)
}

var ErrCollision = fmt.Errorf("collision")

func (d *DB) AddMovie(conn *sqlite.Conn, movie *Movie) error {
	const q = `insert into movies (id, title, original_title, tagline, overview, runtime, genres, languages, release_date, extension, library_path, imported_from_path, is_imported)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	if err := sqlitex.Exec(conn, q, nil,
		movie.ID, movie.Title, movie.OriginalTitle, movie.Tagline, movie.Overview, movie.Runtime, join(movie.Genres), join(movie.Languages), movie.ReleaseDate, movie.Extension, movie.LibraryPath, movie.ImportedFromPath, false,
	); err != nil {
		sqliteError, isSqliteError := err.(sqlite.Error)
		if isSqliteError && sqliteError.Code == sqlite.SQLITE_CONSTRAINT_UNIQUE {
			err = errors.Join(err, ErrCollision)
		}
		return fmt.Errorf("failed to insert movie: %w", err)
	}

	return nil
}

func (d *DB) GetMovieToImport(conn *sqlite.Conn) (*Movie, error) {
	const q = `select id from movies where is_imported = false;`
	var id int64
	onResult := func(stmt *sqlite.Stmt) error {
		id = stmt.ColumnInt64(0)
		return nil
	}
	if err := sqlitex.Exec(conn, q, onResult); err != nil {
		return nil, fmt.Errorf("failed to fetch next movie to import: %w", err)
	}
	if id == 0 {
		return nil, fmt.Errorf("no movies to import")
	}

	movie, err := d.Get(conn, id)
	if err != nil {
		return nil, err
	}

	return movie, nil
}

func (d *DB) MarkMovieAsImported(conn *sqlite.Conn, id int64) error {
	const q = `update movies set is_imported to true where id = ?;`
	if err := sqlitex.Exec(conn, q, nil, id); err != nil {
		return fmt.Errorf("failed to mark movie as imported: %w", err)
	}

	return nil
}

func (d *DB) MovieExistsFromPath(conn *sqlite.Conn, importedFromPath string) (bool, error) {
	const q = `select true from movies where imported_from_path = ?;`
	wasImported := false
	f := func(stmt *sqlite.Stmt) error {
		wasImported = true
		return nil
	}
	if err := sqlitex.Exec(conn, q, f, importedFromPath); err != nil {
		return wasImported, fmt.Errorf("failed to check for movie at path '%s': %w", importedFromPath, err)
	}
	return wasImported, nil
}

func (d *DB) Get(conn *sqlite.Conn, id int64) (*Movie, error) {
	const q = `select id, title, original_title, tagline, overview, runtime, genres, languages, release_date, extension, library_path, imported_from_path from movies where id = ?;`
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
		}
		return nil
	}
	if err := sqlitex.Exec(conn, q, f, id); err != nil {
		return nil, fmt.Errorf("failed to get movie: %w", err)
	}
	return &movie, nil
}
