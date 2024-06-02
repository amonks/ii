package db

import (
	"log"

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

func (db *DB) CreateWatch(entry *letterboxd.Watch) (*Watch, error) {
	watch := entry
	if err := db.Create(watch).Error; err != nil {
		return nil, err
	}
	return watch, nil
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

func (db *DB) AllWatchesMap() (WatchHistory, error) {
	watches, err := db.AllWatches()
	if err != nil {
		return nil, err
	}

	m := map[string][]*Watch{}
	for _, w := range watches {
		key := w.MovieTitle
		existings, has := m[key]
		if !has {
			m[key] = []*Watch{w}
			continue
		}
		if existings[len(existings)-1].Date.After(w.Date) {
			continue
		}
		m[key] = append(existings, w)
	}

	return m, nil
}
