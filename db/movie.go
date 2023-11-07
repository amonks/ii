package db

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"gorm.io/gorm"
	"monks.co/movietagger/tmdb"
)

type Movie struct {
	ID int64 `gorm:"column:id;primaryKey"`

	Title         string
	OriginalTitle string
	Tagline       string
	Overview      string
	Runtime       int64
	Genres        []string `gorm:"serializer:json"`
	Languages     []string `gorm:"serializer:json"`
	ReleaseDate   string

	Extension        string
	LibraryPath      string
	ImportedFromPath string

	ImportedAt string

	TMDBJSON   string `gorm:"column:tmdb_json"`
	PosterPath string

	TMDBCreditsJSON string `gorm:"column:tmdb_credits_json"`
	DirectorName    string

	WriterName string
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

func (m *Movie) IsImported() bool {
	return m.ImportedAt != ""
}

func (d *DB) GetMovieIDToImport() (int64, error) {
	var movie Movie
	if err := d.db.Where("imported_at = ''").First(&movie).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return movie.ID, nil
}

func (d *DB) AddMovie(movie *Movie) error {
	if err := d.db.Create(movie).Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) SaveMovie(movie *Movie) error {
	if err := d.db.Save(movie).Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) AddMovieCredits(movie *Movie, json []byte) error {
	movie.TMDBCreditsJSON = string(json)
	return d.SaveMovie(movie)
}

func (d *DB) AddMovieJSON(movie *Movie, json string) error {
	movie.TMDBJSON = json
	return d.SaveMovie(movie)
}

func (d *DB) AddMovieWriter(movie *Movie, writerName string) error {
	movie.WriterName = writerName
	return d.SaveMovie(movie)
}

func (d *DB) AddMovieDirector(movie *Movie, directorName string) error {
	movie.DirectorName = directorName
	return d.SaveMovie(movie)
}

func (d *DB) AddMoviePoster(movie *Movie, posterPath string) error {
	movie.PosterPath = posterPath
	return d.SaveMovie(movie)
}

func (d *DB) SetMovieImportedAt(movie *Movie, importedAt time.Time) error {
	movie.ImportedAt = importedAt.Format(time.DateTime)
	return d.SaveMovie(movie)
}

func (d *DB) ReplaceMovie(movie *Movie, path string) error {
	movie.ImportedAt = time.Now().Format(time.DateTime)
	movie.ImportedFromPath = path
	return d.SaveMovie(movie)
}

func (d *DB) DeleteMovie(movie *Movie) error {
	if err := d.db.Delete(movie).Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) MovieExistsFromPath(importedFromPath string) (bool, error) {
	var movie Movie
	if err := d.db.Where("imported_from_path = ?", importedFromPath).First(&movie).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return movie.ID != 0, nil
}

func (d *DB) AllMovies() ([]*Movie, error) {
	movies := []*Movie{}
	tx := d.db.Table("movies").Find(&movies)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return movies, nil
}

func (m *Movie) HasPoster() bool {
	return m.PosterPath != ""
}

func (m *Movie) PosterURL() string {
	return fmt.Sprintf("poster?id=%d", m.ID)
}

func (d *DB) GetMovie(id int64) (*Movie, error) {
	var movie Movie
	if err := d.db.Where(&Movie{ID: id}).First(&movie).Error; err != nil {
		return nil, err
	}
	return &movie, nil
}
