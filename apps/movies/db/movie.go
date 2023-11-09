package db

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"gorm.io/gorm"
	"monks.co/pkg/tmdb"
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
	WriterName      string

	MetacriticRating    int
	MetacriticURL       string
	MetacriticValidated bool
}

func (db *DB) CreateMovie(m *tmdb.Movie, importedFromPath string) (*Movie, error) {
	if importedFromPath == "" {
		return nil, fmt.Errorf("invalid path")
	}

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

	if err := db.Create(&movie).Error; err != nil {
		return nil, err
	}

	db.notify(&movie)

	return &movie, nil
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
	if err := d.Where("imported_at = ''").First(&movie).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return movie.ID, nil
}

func (d *DB) AddMovieCredits(movie *Movie, json []byte) error {
	if err := d.Model(&Movie{}).
		Where("id = ?", movie.ID).
		Updates(Movie{TMDBCreditsJSON: string(json)}).
		Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) AddMovieJSON(movie *Movie, json string) error {
	if err := d.Model(&Movie{}).
		Where("id = ?", movie.ID).
		Updates(Movie{TMDBJSON: string(json)}).
		Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) AddMovieWriter(movie *Movie, writerName string) error {
	if err := d.Model(&Movie{}).
		Where("id = ?", movie.ID).
		Updates(Movie{WriterName: writerName}).
		Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) AddMovieDirector(movie *Movie, directorName string) error {
	if err := d.Model(&Movie{}).
		Where("id = ?", movie.ID).
		Updates(Movie{DirectorName: directorName}).
		Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) SetMovieMetacriticValidated(movie *Movie, valid bool) error {
	if err := d.Model(&Movie{}).
		Where("id = ?", movie.ID).
		Update("metacritic_validated", valid).
		Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) AddMovieRating(movie *Movie, score int, metacriticURL string) error {
	if err := d.Model(&Movie{}).
		Where("id = ?", movie.ID).
		Updates(map[string]interface{}{
			"metacritic_rating": score,
			"metacritic_url": metacriticURL,
			"metacritic_validated": false,
		}).
		Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) AddMoviePoster(movie *Movie, posterPath string) error {
	if err := d.Model(&Movie{}).
		Where("id = ?", movie.ID).
		Updates(Movie{PosterPath: posterPath}).
		Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) SetMovieImportedAt(movie *Movie, importedAt time.Time) error {
	if err := d.Model(&Movie{}).
		Where("id = ?", movie.ID).
		Updates(Movie{ImportedAt: importedAt.Format(time.DateTime)}).
		Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) ReplaceMovie(movie *Movie, path string) error {
	if err := d.Model(&Movie{}).
		Where("id = ?", movie.ID).
		Updates(Movie{
			ImportedAt:       time.Now().Format(time.DateTime),
			ImportedFromPath: path,
		}).
		Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) DeleteMovie(movie *Movie) error {
	if err := d.Delete(movie).Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) MovieExistsFromPath(importedFromPath string) (bool, error) {
	var movie Movie
	if err := d.Where("imported_from_path = ?", importedFromPath).First(&movie).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return movie.ID != 0, nil
}

func (d *DB) AllMovies() ([]*Movie, error) {
	movies := []*Movie{}
	if err := d.Table("movies").Find(&movies).Error; err != nil {
		return nil, err
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
	if err := d.Where(&Movie{ID: id}).First(&movie).Error; err != nil {
		return nil, err
	}
	return &movie, nil
}
