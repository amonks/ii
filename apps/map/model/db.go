package model

import "monks.co/pkg/database"

type Model struct {
	*database.DB
}

func NewModel() (*Model, error) {
	db, err := database.OpenFromDataFolder("map")
	if err != nil {
		return nil, err
	}
	return &Model{db}, nil
}
