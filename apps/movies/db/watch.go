package db

import (
	"log"

	"monks.co/pkg/letterboxd"
)

type Watch = letterboxd.Watch

func (db *DB) CreateWatch(entry *letterboxd.Watch) (*Watch, error) {
	watch := entry
	if err := db.Create(watch).Error; err != nil {
		return nil, err
	}
	return watch, nil
}

type Key struct {
	title string
	year  int
}

func (db *DB) AllWatches() ([]*Watch, error) {
	watches := []*Watch{}
	if err := db.Table("watches").
		Order("date desc").
		Find(&watches).
		Error; err != nil {
		return nil, err
	}
	log.Printf("fetch %d watches", len(watches))
	return watches, nil
}

func (db *DB) AllWatchesMap() (map[Key]*Watch, error) {
	watches, err := db.AllWatches()
	if err != nil {
		return nil, err
	}

	m := map[Key]*Watch{}
	for _, w := range watches {
		key := Key{w.MovieTitle, w.MovieReleaseYear}
		existing, has := m[key]
		if !has {
			m[key] = w
			continue
		}
		if existing.Date.After(w.Date) {
			continue
		}
		m[key] = w
	}

	return m, nil
}
