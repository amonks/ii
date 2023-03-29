package mastodon

import (
	"embed"
	"html/template"

	"monks.co/dbserver"
	"monks.co/util"
)

var (
	//go:embed templates/*
	files     embed.FS
	templates map[string]*template.Template
)

func init() {
	ts, err := util.ReadTemplates(files, "templates")
	if err != nil {
		panic(err)
	}
	templates = ts
}

type server struct {
	*dbserver.DBServer
	model  *model
	client *Mastodon
}

func Server() *server {
	s := &server{
		DBServer: dbserver.New("mastodon"),
		model:    NewModel(),
	}

	s.HandleFunc("/mastodon/index.css", s.StaticServer("./mastodon/static/"))

	s.Init(s.model.migrate)
	return s
}
