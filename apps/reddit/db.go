package main

import (
	"monks.co/pkg/database"
)

type model struct {
	*database.DB
}

func NewModel() (*model, error) {
	db, err := database.Open("golink.db")
	if err != nil {
		return nil, err
	}
	return &model{db}, nil
}

func (m *model) getPosts(limit, offset int) ([]*Post, error) {
	posts := []*Post{}
	if err := m.DB.Table("posts").Find(&posts).Offset(offset).Limit(limit).Error; err != nil {
		return nil, err
	}
	return posts, nil
}
