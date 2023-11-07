package main

import (
	"time"

	"monks.co/pkg/database"
)

type Shortening struct {
	URL       string `gorm:"url"`
	Key       string `gorm:"primaryKey"`
	CreatedAt *time.Time
}

type model struct {
	*database.DB
}

func NewModel() (*model, error) {
	db, err := database.OpenFromDataFolder("golink")
	if err != nil {
		return nil, err
	}
	return &model{db}, nil
}

func (m *model) List() ([]Shortening, error) {
	ss := []Shortening{}
	if err := m.DB.Table("shortenings").
		Find(&ss).
		Error; err != nil {
		return nil, err
	}
	return ss, nil
}

func (m *model) Get(key string) (string, error) {
	s := Shortening{}
	if err := m.DB.Table("shortenings").
		Where("key = ?", key).
		Find(&s).
		Error; err != nil {
		return "", err
	}
	return s.URL, nil
}

func (m *model) Set(key, url string) error {
	if err := m.DB.Save(&Shortening{
		Key: key,
		URL: url,
	}).Error; err != nil {
		return err
	}
	return nil
}

func (m *model) Delete(key string) error {
	if err := m.DB.Delete(Shortening{
		Key: key,
	}).Error; err != nil {
		return err
	}
	return nil
}
