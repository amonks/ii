package main

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm/clause"
	"monks.co/pkg/database"
	"monks.co/pkg/lastfm"
)

type DB struct {
	*database.DB
}

type Artist struct {
	URL  string `gorm:"column:url"`
	Name string `gorm:"column:name"`
	MBID string `gorm:"column:mbid"`
}

type Album struct {
	Name      string `gorm:"column:name"`
	MBID      string `gorm:"column:mbid"`
	ArtistURL string `gorm:"column:artist_url"`
}

type Track struct {
	URL  string `gorm:"column:url"`
	Name string `gorm:"column:name"`
	MBID string `gorm:"column:mbid"`

	ArtistURL string `gorm:"column:artist_url"`

	AlbumMBID string `gorm:"column:album_mbid"`
	AlbumName string `gorm:"column:album_name"`
}

type Scrobble struct {
	Date     time.Time `gorm:"column:date"`
	TrackURL string    `gorm:"column:track_url"`
}

var ErrDuplicate = errors.New("duplicate")
var ErrStillListening = errors.New("still listening")

func (db *DB) AddScrobble(sc *lastfm.Scrobble) error {
	date := sc.Time()
	if date.IsZero() {
		return ErrStillListening
	}

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&Artist{
		URL:  sc.Artist.URL,
		Name: sc.Artist.Name,
		MBID: sc.Artist.MBID,
	}).Error; err != nil {
		return err
	}

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&Album{
		Name:      sc.Album.Text,
		MBID:      sc.Album.MBID,
		ArtistURL: sc.Artist.URL,
	}).Error; err != nil {
		return err
	}

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&Track{
		URL:  sc.URL,
		Name: sc.Name,
		MBID: sc.MBID,

		ArtistURL: sc.Artist.URL,
		AlbumMBID: sc.Album.MBID,
		AlbumName: sc.Album.Text,
	}).Error; err != nil {
		return err
	}

	scrobble := &Scrobble{
		Date:     date,
		TrackURL: sc.URL,
	}
	if err := db.Create(scrobble).Error; err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return err
	} else if err != nil {
		return ErrDuplicate
	}

	return nil
}

func NewDB() (*DB, error) {
	db, err := database.OpenFromDataFolder("scrobbles")
	if err != nil {
		return nil, err
	}
	if err := db.Exec(`
		create table if not exists artists (
			url text primary key,
			name text,
			mbid text
		);

		create table if not exists albums (
			name text,
			mbid text,
			artist_url text references artists(url),

			primary key (name, mbid, artist_url)
		);

		create table if not exists tracks (
			url text primary key,
			name text,
			mbid text,

			artist_url text references artists(url),

			album_mbid text references albums(mbid),
			album_name text references albums(name)
		);

		create table if not exists scrobbles (
			date datetime,
			track_url text references tracks(url),

			primary key (track_url, date)
		);
	`).Error; err != nil {
		return nil, err
	}

	return &DB{db}, nil
}
