package dogs

import (
	"path/filepath"

	"monks.co/pkg/database"
)

type Entry struct {
	Number        int
	Date          string
	Count         float64
	Eater         string
	PhotoURL      string
	PhotoFilename string
	Notes         string
	Wordcount     int
}

type DB struct {
	*database.DB
}

func NewDB(dir string) (*DB, error) {
	db, err := database.Open(filepath.Join(dir, "dogs.db"))
	if err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

type QueryOptions struct {
	Combatants []string
	Sort       string
}

func (db *DB) All(opts QueryOptions) ([]*Entry, error) {
	entries := []*Entry{}

	q := db.DB.Table("entries")

	if len(opts.Combatants) > 0 {
		q = q.Where("eater in ?", opts.Combatants)
	}

	q = q.Order(opts.Sort)

	if err := q.Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}
