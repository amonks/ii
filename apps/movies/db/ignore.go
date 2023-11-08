package db

import (
	"errors"

	"gorm.io/gorm"
)

type Ignore struct {
	ImportedFromPath string `gorm:"column:path;primaryKey"`
	Type             MediaType
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
	if err := db.Create(&Ignore{ImportedFromPath: path, Type: mediaType}).Error; err != nil {
		return err
	}
	return nil
}
