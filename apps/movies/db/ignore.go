package db

import (
	"errors"

	"gorm.io/gorm"
)

type Ignore struct {
	ImportedFromPath string `gorm:"column:path;primaryKey"`
}

func (db *DB) PathIsIgnored(path string) (bool, error) {
	var ignore Ignore
	tx := db.Where(&Ignore{ImportedFromPath: path}).First(&ignore)
	if err := tx.Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (db *DB) IgnorePath(path string) error {
	if err := db.Create(&Ignore{ImportedFromPath: path}).Error; err != nil {
		return err
	}
	return nil
}
