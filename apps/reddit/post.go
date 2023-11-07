package main

import (
	"html/template"
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

func (p *Post) Embed() template.HTML {
	if p.Filetype == nil {
		return "no"
	}
	src := strings.Replace(*p.Archivepath, archivePath, "media/", 1)
	switch *p.Filetype {
	case ".gif":
		fallthrough
	case ".jpg":
		fallthrough
	case ".png":
		tmpl, _ := template.New("gif").Parse(`<img src="{{.Src}}" />`)
		w := strings.Builder{}
		tmpl.Execute(&w, struct{ Src string }{Src: src})
		return template.HTML(w.String())
	case ".mp4":
		tmpl, _ := template.New("mp4").Parse(`<video controls autoplay loop><source src="{{ .Src}}" /></video>`)
		w := strings.Builder{}
		tmpl.Execute(&w, struct{ Src string }{Src: src})
		return template.HTML(w.String())
	default:
		return template.HTML("unexpected filetype: " + *p.Filetype)
	}
}

