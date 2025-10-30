package db

import (
	"errors"

	"gorm.io/gorm"
)

type IgnoreType int

const (
	IgnoreTypeShow    IgnoreType = 1 // Ignore entire show directory
	IgnoreTypeEpisode IgnoreType = 2 // Ignore specific episode file
)

type Ignore struct {
	ImportedFromPath string `gorm:"column:path;primaryKey"`
	Type             MediaType
	IgnoreType       IgnoreType
}

func (db *DB) PathIsIgnored(mediaType MediaType, path string) (bool, error) {
	var ignore Ignore
	tx := db.Where(&Ignore{ImportedFromPath: path, Type: mediaType}).First(&ignore)
	if err := tx.Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (db *DB) IgnorePath(mediaType MediaType, path string) error {
	if err := db.Create(&Ignore{ImportedFromPath: path, Type: mediaType, IgnoreType: IgnoreTypeShow}).Error; err != nil {
		return err
	}
	return nil
}

// IgnoreEpisode ignores a specific episode file
func (db *DB) IgnoreEpisode(path string) error {
	if err := db.Create(&Ignore{ImportedFromPath: path, Type: MediaTypeTV, IgnoreType: IgnoreTypeEpisode}).Error; err != nil {
		return err
	}
	return nil
}

// EpisodeIsIgnored checks if a specific episode file is ignored
func (db *DB) EpisodeIsIgnored(path string) (bool, error) {
	var ignore Ignore
	tx := db.Where(&Ignore{ImportedFromPath: path, Type: MediaTypeTV, IgnoreType: IgnoreTypeEpisode}).First(&ignore)
	if err := tx.Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (db *DB) AllIgnores() ([]*Ignore, error) {
	var ignores []*Ignore
	if err := db.Find(&ignores).Error; err != nil {
		return nil, err
	}
	return ignores, nil
}

func (db *DB) DeleteIgnore(path string, mediaType MediaType) error {
	if err := db.Where(&Ignore{ImportedFromPath: path, Type: mediaType}).Delete(&Ignore{}).Error; err != nil {
		return err
	}
	return nil
}
