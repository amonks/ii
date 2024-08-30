package db

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"monks.co/pkg/letterboxd"
)

type WatchHistory map[string][]*Watch

func (wl WatchHistory) LastWatch(title string) *Watch {
	watches, hasWatched := wl[title]
	if !hasWatched {
		return nil
	}
	return watches[len(watches)-1]
}

type Watch = letterboxd.Watch

// virtual fts4 table
type WatchTitle struct {
	LetterboxdURL string // references watches.letterboxd_url
	Title         string // references watches.movie_title
}

// join
type MovieWatch struct {
	ID            int64  // references movies.id
	LetterboxdURL string // references watches.letterboxd_url
}

func (db *DB) CreateWatch(watch *letterboxd.Watch) (*Watch, error) {
	if err := db.Table("watches").Create(watch).Error; err != nil {
		return nil, err
	}
	if err := db.Table("watch_titles").Create(&WatchTitle{LetterboxdURL: watch.LetterboxdURL, Title: watch.MovieTitle}).Error; err != nil {
		return nil, err
	}
	if movie, err := db.FindMovieByTitle(watch.MovieTitle); err != nil {
		return nil, err
	} else if movie != nil {
		if err := db.Table("movie_watches").Create(&MovieWatch{ID: movie.ID, LetterboxdURL: watch.LetterboxdURL}).Error; err != nil {
			return nil, err
		}
	}
	return watch, nil
}

func (db *DB) Watches(movieID int64) ([]Watch, error) {
	movieWatches := []MovieWatch{}
	if err := db.Table("movie_watches").Where(&MovieWatch{ID: movieID}).Find(&movieWatches).Error; err != nil {
		return nil, err
	}
	var watches []Watch
	for _, mw := range movieWatches {
		watch := &Watch{}
		if err := db.Table("watches").Where(&Watch{LetterboxdURL: mw.LetterboxdURL}).Find(watch).Error; err != nil {
			return nil, err
		}
		watches = append(watches, *watch)
	}
	return watches, nil
}

func (db *DB) FindWatchByTitle(title string) (*Watch, error) {
	sanitized := nonAlpha.ReplaceAllString(title, " ")
	var watchTitle WatchTitle
	if err := db.Where("title match ?", sanitized).First(&watchTitle).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var watch Watch
	if err := db.Where(&Watch{LetterboxdURL: watchTitle.LetterboxdURL}).First(&watch).Error; err != nil {
		return nil, err
	}
	return &watch, nil
}

func (db *DB) PopulateMovieWatches() error {
	watches := []*Watch{}
	if err := db.Table("watches").Find(&watches).Error; err != nil {
		return fmt.Errorf("err finding watch: %w", err)
	}
	for _, watch := range watches {
		if movie, err := db.FindMovieByTitle(watch.MovieTitle); err != nil {
			return fmt.Errorf("err finding movie: %w", err)
		} else if movie != nil {
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).
				Create(&MovieWatch{ID: movie.ID, LetterboxdURL: watch.LetterboxdURL}).
				Error; err != nil {
				return fmt.Errorf("err creating moviewatch: %w", err)
			}
		} else {
			fmt.Println("no matching movie for ", watch.MovieTitle)
		}
	}
	return nil
}
