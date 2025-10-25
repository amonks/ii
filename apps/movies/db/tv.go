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

var illegalCharForTVFilename = regexp.MustCompile(`[^a-zA-Z0-9\.\- ]+`)

// TVShow represents a TV show in the database
type TVShow struct {
	ID           int64 `gorm:"primaryKey"`
	Name         string
	OriginalName string
	Overview     string
	Status       string
	FirstAirDate string
	LastAirDate  string
	Genres       []string `gorm:"serializer:json"`
	Languages    []string `gorm:"serializer:json"`

	LibraryPath string
	ImportedAt  string

	TMDBJSON        string `gorm:"column:tmdb_json"`
	PosterPath      string
	TMDBCreditsJSON string `gorm:"column:tmdb_credits_json"`

	// Not persisted
	Seasons []TVSeason `gorm:"-:all"`
}

// TVSeason represents a season of a TV show in the database
type TVSeason struct {
	ID           int64 `gorm:"-:all"`
	ShowID       int64 `gorm:"primaryKey;column:show_id"`
	SeasonNumber int   `gorm:"primaryKey"`
	Name         string
	Overview     string
	EpisodeCount int
	AirDate      string
	PosterPath   string
	ImportedAt   string
	TMDBJSON     string `gorm:"column:tmdb_json"`
	LibraryPath  string

	// Not persisted
	Episodes []TVEpisode `gorm:"-:all"`
}

// TVEpisode represents a TV episode in the database
type TVEpisode struct {
	ID            int64 `gorm:"-:all"`
	ShowID        int64 `gorm:"primaryKey;column:show_id"`
	SeasonNumber  int   `gorm:"primaryKey"`
	EpisodeNumber int   `gorm:"primaryKey"`
	Name          string
	Overview      string
	Runtime       int64
	AirDate       string
	StillPath     string

	Extension        string
	ImportedFromPath string
	LibraryPath      string
	IsCopied         bool
	ImportedAt       string

	TMDBJSON string `gorm:"column:tmdb_json"`
}

// TVShowTitle is for the FTS table
type TVShowTitle struct {
	ID   int64  // references tv_shows.id
	Name string // references tv_shows.name
}

// TVSearchResult represents a TV show search result from TMDB
type TVSearchResult struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	FirstAirDate string `json:"first_air_date"`
}

func (s *TVShow) BuildLibraryPath() string {
	// Return the folder path for the TV show with release year
	name := illegalCharForTVFilename.ReplaceAllString(s.Name, "-")
	year := ""
	if s.FirstAirDate != "" && len(s.FirstAirDate) >= 4 {
		year = s.FirstAirDate[:4]
		return fmt.Sprintf("%s (%s)", name, year)
	}
	return name
}

func (s *TVSeason) BuildLibraryPath(showPath string) string {
	// Return the folder path for the season within the show directory
	return filepath.Join(showPath, fmt.Sprintf("Season %d", s.SeasonNumber))
}

func (e *TVEpisode) BuildLibraryPath(seasonPath string) string {
	// Return the file path for the episode within the season directory
	// Sanitize the episode name to remove illegal filesystem characters
	episodeName := illegalCharForTVFilename.ReplaceAllString(e.Name, "-")
	return filepath.Join(seasonPath, fmt.Sprintf("S%02dE%02d - %s%s",
		e.SeasonNumber, e.EpisodeNumber, episodeName, e.Extension))
}

// CreateTVShow adds a new TV show to the database
func (db *DB) CreateTVShow(t *tmdb.TVShow) (*TVShow, error) {
	var genres []string
	var languages []string
	for _, genre := range t.Genres {
		genres = append(genres, genre.Name)
	}
	for _, language := range t.Languages {
		languages = append(languages, language.Name)
	}

	show := TVShow{
		ID:           t.ID,
		Name:         t.Name,
		OriginalName: t.OriginalName,
		Overview:     t.Overview,
		Status:       t.Status,
		FirstAirDate: t.FirstAirDate,
		LastAirDate:  t.LastAirDate,
		Genres:       genres,
		Languages:    languages,
		ImportedAt:   time.Now().Format(time.DateTime),
		TMDBJSON:     t.TMDBJSON,
	}
	show.LibraryPath = show.BuildLibraryPath()

	if err := db.Table("tv_shows").Create(&show).Error; err != nil {
		return nil, err
	}
	if err := db.Table("tv_show_titles").Create(&TVShowTitle{ID: show.ID, Name: show.Name}).Error; err != nil {
		return nil, err
	}

	return &show, nil
}

// CreateTVSeason adds a new season to the database
func (db *DB) CreateTVSeason(showID int64, s *tmdb.Season) (*TVSeason, error) {
	season := TVSeason{
		ID:           s.ID,
		ShowID:       showID,
		SeasonNumber: s.SeasonNumber,
		Name:         s.Name,
		Overview:     s.Overview,
		EpisodeCount: s.EpisodeCount,
		AirDate:      s.AirDate,
		ImportedAt:   time.Now().Format(time.DateTime),
	}

	// Get show path for constructing the season path
	var show TVShow
	if err := db.Table("tv_shows").Where("id = ?", showID).First(&show).Error; err != nil {
		return nil, err
	}

	season.LibraryPath = season.BuildLibraryPath(show.LibraryPath)

	if err := db.Table("tv_seasons").Create(&season).Error; err != nil {
		return nil, err
	}

	// Notify subscribers about the new TV season
	db.notifyTV(&season)

	return &season, nil
}

// CreateTVEpisode adds a new episode to the database
func (db *DB) CreateTVEpisode(showID int64, e *tmdb.Episode, importedFromPath string) (*TVEpisode, error) {
	episode := TVEpisode{
		ID:               e.ID,
		ShowID:           showID,
		SeasonNumber:     e.SeasonNumber,
		EpisodeNumber:    e.EpisodeNumber,
		Name:             e.Name,
		Overview:         e.Overview,
		Runtime:          e.Runtime,
		AirDate:          e.AirDate,
		StillPath:        e.StillPath,
		Extension:        filepath.Ext(importedFromPath),
		ImportedFromPath: importedFromPath,
		ImportedAt:       time.Now().Format(time.DateTime),
	}

	// Get season path for constructing the episode path
	var season TVSeason
	if err := db.Table("tv_seasons").Where("show_id = ? AND season_number = ?", showID, e.SeasonNumber).First(&season).Error; err != nil {
		return nil, err
	}

	episode.LibraryPath = episode.BuildLibraryPath(season.LibraryPath)

	if err := db.Table("tv_episodes").Create(&episode).Error; err != nil {
		return nil, err
	}

	return &episode, nil
}

// GetTVShow retrieves a TV show by ID
func (db *DB) GetTVShow(id int64) (*TVShow, error) {
	var show TVShow
	if err := db.Table("tv_shows").Where("id = ?", id).First(&show).Error; err != nil {
		return nil, err
	}

	// Load seasons
	var seasons []TVSeason
	if err := db.Table("tv_seasons").Where("show_id = ?", id).Find(&seasons).Error; err != nil {
		return nil, err
	}
	show.Seasons = seasons

	return &show, nil
}

// GetTVSeason retrieves a season by show ID and season number
func (db *DB) GetTVSeason(showID int64, seasonNumber int) (*TVSeason, error) {
	var season TVSeason
	if err := db.Table("tv_seasons").Where("show_id = ? AND season_number = ?", showID, seasonNumber).First(&season).Error; err != nil {
		return nil, err
	}

	// Load episodes
	var episodes []TVEpisode
	if err := db.Table("tv_episodes").Where("show_id = ? AND season_number = ?", showID, seasonNumber).Find(&episodes).Error; err != nil {
		return nil, err
	}
	season.Episodes = episodes

	return &season, nil
}

// GetTVEpisode retrieves an episode by show ID, season number, and episode number
func (db *DB) GetTVEpisode(showID int64, seasonNumber, episodeNumber int) (*TVEpisode, error) {
	var episode TVEpisode
	if err := db.Table("tv_episodes").Where("show_id = ? AND season_number = ? AND episode_number = ?",
		showID, seasonNumber, episodeNumber).First(&episode).Error; err != nil {
		return nil, err
	}
	return &episode, nil
}

// AllTVShows retrieves all TV shows from the database
func (db *DB) AllTVShows() ([]*TVShow, error) {
	var shows []*TVShow
	if err := db.Table("tv_shows").Find(&shows).Error; err != nil {
		return nil, err
	}

	// Load seasons for each show
	for _, show := range shows {
		var seasons []TVSeason
		if err := db.Table("tv_seasons").Where("show_id = ?", show.ID).Find(&seasons).Error; err != nil {
			return nil, err
		}
		show.Seasons = seasons
	}

	return shows, nil
}

// GetTVShows is an alias for AllTVShows, used by stubquerygenerator
func (db *DB) GetTVShows() ([]*TVShow, error) {
	return db.AllTVShows()
}

// AddTVShowPoster adds a poster path to a TV show
func (db *DB) AddTVShowPoster(show *TVShow, posterPath string) error {
	if err := db.Table("tv_shows").Where("id = ?", show.ID).
		Updates(map[string]interface{}{"poster_path": posterPath}).Error; err != nil {
		return err
	}
	return nil
}

// SetTVEpisodeCopied marks an episode as copied to the library
func (db *DB) SetTVEpisodeCopied(episode *TVEpisode) error {
	if err := db.Table("tv_episodes").Where("show_id = ? AND season_number = ? AND episode_number = ?",
		episode.ShowID, episode.SeasonNumber, episode.EpisodeNumber).
		Updates(map[string]interface{}{"is_copied": true}).Error; err != nil {
		return err
	}
	return nil
}

// TVShowExistsFromPath checks if a TV show exists in the database with the given import path
func (db *DB) TVShowExistsFromPath(importedFromPath string) (bool, error) {
	var episode TVEpisode
	if err := db.Table("tv_episodes").
		Where("imported_from_path = ?", importedFromPath).
		First(&episode).
		Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// TVShowExistsByPathPrefix checks if any TV episodes exist with paths starting with the given prefix
func (db *DB) TVShowExistsByPathPrefix(pathPrefix string) (bool, error) {
	var episode TVEpisode
	if err := db.Table("tv_episodes").
		Where("imported_from_path LIKE ?", pathPrefix+"%").
		First(&episode).
		Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// FindTVShowByName finds a TV show by name using FTS
func (db *DB) FindTVShowByName(name string) (*TVShow, error) {
	sanitized := nonAlpha.ReplaceAllString(name, " ")
	var showTitle TVShowTitle
	if err := db.Table("tv_show_titles").Where("name match ?", sanitized).First(&showTitle).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return db.GetTVShow(showTitle.ID)
}

// TVEpisodeExistsFromPath checks if an episode already exists with the given path
func (db *DB) TVEpisodeExistsFromPath(importedFromPath string) (bool, error) {
	var episode TVEpisode
	if err := db.Table("tv_episodes").
		Where("imported_from_path = ?", importedFromPath).
		First(&episode).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// GetTVEpisodeIDToCopy gets the next TV episode to copy
func (db *DB) GetTVEpisodeIDToCopy() (int64, int, int, error) {
	var episode TVEpisode
	if err := db.Table("tv_episodes").
		Where("is_copied = false").
		First(&episode).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, 0, nil
	} else if err != nil {
		return 0, 0, 0, err
	}
	return episode.ShowID, episode.SeasonNumber, episode.EpisodeNumber, nil
}

// GetTVShowSeasons gets all seasons for a TV show
func (db *DB) GetTVShowSeasons(showID int64) ([]*TVSeason, error) {
	var seasons []*TVSeason
	if err := db.Table("tv_seasons").
		Where("show_id = ?", showID).
		Find(&seasons).Error; err != nil {
		return nil, err
	}
	return seasons, nil
}

// GetTVSeasonEpisodes gets all episodes for a TV season
func (db *DB) GetTVSeasonEpisodes(showID int64, seasonNumber int) ([]*TVEpisode, error) {
	var episodes []*TVEpisode
	if err := db.Table("tv_episodes").
		Where("show_id = ? AND season_number = ?", showID, seasonNumber).
		Find(&episodes).Error; err != nil {
		return nil, err
	}
	return episodes, nil
}

// UpdateTVEpisodePath updates the path for an existing episode
func (db *DB) UpdateTVEpisodePath(episode *TVEpisode, newPath string) error {
	return db.Table("tv_episodes").
		Where("show_id = ? AND season_number = ? AND episode_number = ?",
			episode.ShowID, episode.SeasonNumber, episode.EpisodeNumber).
		Updates(map[string]interface{}{
			"imported_from_path": newPath,
			"is_copied":          false,
		}).Error
}
