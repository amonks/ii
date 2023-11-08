package main

import (
	"strings"
)

type Post struct {
	Name      string `gorm:"primaryKey"`
	Title     string
	Author    string
	Subreddit string
	Url       string
	Permalink string

	Json *[]byte

	Status      string
	Filetype    *string
	Archivepath *string
}

func (p *Post) Src() string {
	return strings.Replace(*p.Archivepath, archivePath, "media/", 1)
}
