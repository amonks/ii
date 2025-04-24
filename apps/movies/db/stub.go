package db

import (
	"errors"

	"gorm.io/gorm"
	"monks.co/pkg/tmdb"
)

type Stub struct {
	ImportedFromPath string `gorm:"primaryKey"`
	Type             MediaType
	Year             string
	Query            string
	Results          []tmdb.SearchResult `gorm:"serializer:json"`
	TVResults        []TVSearchResult    `gorm:"serializer:json"`
	EpisodeFiles     []string            `gorm:"serializer:json"` // For TV shows, store the list of episode files
	SeasonNumber     int                 // For TV shows, store the identified season number
	SearchStatus     string              // Track search status: "pending", "searching", "needs_manual", "complete"
}

type Result struct {
	ID    string
	Title string
	Year  string
}

func (d *DB) AllStubs() ([]*Stub, error) {
	stubs := []*Stub{}
	if err := d.Table("stubs").Find(&stubs).Error; err != nil {
		return nil, err
	}
	return stubs, nil
}

func (d *DB) GetStub(importedFromPath string) (*Stub, error) {
	stub := &Stub{ImportedFromPath: importedFromPath}
	if err := d.Table("stubs").Find(stub).Error; err != nil {
		return nil, err
	}
	return stub, nil
}

func (d *DB) CreateStub(mediaType MediaType, importedFromPath string) (*Stub, error) {
	stub := &Stub{
		ImportedFromPath: importedFromPath,
		Type:             mediaType,
	}
	if err := d.Create(stub).Error; err != nil {
		return nil, err
	}
	return stub, nil
}

func (d *DB) SaveStub(stub *Stub) error {
	if err := d.Save(stub).Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) DeleteStub(stub *Stub) error {
	if err := d.Delete(stub).Error; err != nil {
		return err
	}
	return nil
}

func (d *DB) StubExistsFromPath(mediaType MediaType, importedFromPath string) (bool, error) {
	var stub Stub
	if err := d.Table("stubs").
		Where(Stub{ImportedFromPath: importedFromPath, Type: mediaType}).
		First(&stub).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return stub.ImportedFromPath != "", nil
}

// CountStubsByType counts the number of stubs of a specific media type
func (d *DB) CountStubsByType(mediaType MediaType) (int, error) {
	var count int64
	if err := d.Table("stubs").Where("type = ?", mediaType).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}
