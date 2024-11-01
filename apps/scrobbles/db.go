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
	AlbumName string `gorm:"column:album_name"`
}

type Scrobble struct {
	Date     time.Time `gorm:"column:date"`
	TrackURL string    `gorm:"column:track_url"`

	Track  Track  `gorm:"-"`
	Artist Artist `gorm:"-"`
	Album  Album  `gorm:"-"`
}

var ErrDuplicate = errors.New("duplicate")
var ErrStillListening = errors.New("still listening")

func (db *DB) GetScrobbles(limit, offset int) ([]Scrobble, error) {
	type Result struct {
		ScrobbleDate time.Time
		TrackName    string
		TrackMBID    string `gorm:"column:track_mbid"`
		TrackURL     string `gorm:"column:track_url"`
		AlbumName    string `gorm:"column:album_name"`
		AlbumMBID    string `gorm:"column:album_mbid"`
		ArtistURL    string `gorm:"column:artist_url"`
		ArtistName   string `gorm:"column:artist_name"`
		ArtistMBID   string `gorm:"column:artist_mbid"`
	}
	var results []Result
	if err := db.
		Select([]string{
			"scrobbles.date as scrobble_date",
			"tracks.name as track_name",
			"tracks.mbid as track_mbid",
			"tracks.url as track_url",
			"albums.name as album_name",
			"albums.mbid as album_mbid",
			"artists.url as artist_url",
			"artists.name as artist_name",
			"artists.mbid as artist_mbid",
		}).
		Table("scrobbles").
		Joins("left join tracks on scrobbles.track_url = tracks.url").
		Joins("left join artists on tracks.artist_url = artists.url").
		Joins("left join albums on tracks.artist_url = albums.artist_url and tracks.album_name = albums.name").
		Order("scrobbles.date desc").
		Limit(limit).
		Offset(offset).
		Find(&results).
		Error; err != nil {
		return nil, err
	}
	scrobbles := make([]Scrobble, len(results))
	for i, result := range results {
		scrobbles[i] = Scrobble{
			Date:     result.ScrobbleDate,
			TrackURL: result.TrackURL,

			Track: Track{
				URL:       result.TrackURL,
				Name:      result.TrackName,
				MBID:      result.TrackMBID,
				ArtistURL: result.ArtistURL,
				AlbumName: result.AlbumName,
			},
			Artist: Artist{
				URL:  result.ArtistURL,
				Name: result.ArtistName,
				MBID: result.ArtistMBID,
			},
			Album: Album{
				ArtistURL: result.ArtistURL,
				Name:      result.AlbumName,
				MBID:      result.AlbumMBID,
			},
		}
	}
	return scrobbles, nil
}

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
		create index if not exists artists_by_mbid on artists ( mbid );

		create table if not exists albums (
			name text,
			mbid text,
			artist_url text references artists(url),

			primary key (artist_url, name)
		);

		create table if not exists tracks (
			url text primary key,
			name text,
			mbid text,

			artist_url text references artists(url),
			album_name text references albums(name)
		);
		create index if not exists tracks_by_artist_url on tracks ( artist_url );
		create index if not exists artist_url_and_album_name on tracks ( artist_url, album_name );

		create table if not exists scrobbles (
			date datetime,
			track_url text references tracks(url),

			primary key (date, track_url)
		);
		create index if not exists scrobbles_by_track_url on scrobbles ( track_url );
	`).Error; err != nil {
		return nil, err
	}

	return &DB{db}, nil
}
