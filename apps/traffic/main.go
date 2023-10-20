package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"net/http"

	"gorm.io/gorm"
	"monks.co/pkg/gzip"
	"monks.co/pkg/traffic"
	"monks.co/pkg/util"
)

var port = flag.Int("port", 3000, "port")

func main() {
	flag.Parse()
	db, err := traffic.Open()
	if err != nil {
		panic(err)
	}
	app := &App{db}
	http.Handle("/", gzip.GzipHandler(app))
	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	fmt.Println("listening on", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}

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

type App struct{ db *gorm.DB }

func (app *App) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var requests []traffic.Request
	if tx := app.db.Order("created_at desc").Find(&requests); tx.Error != nil {
		util.HTTPError("traffic", w, req, 500, "failed to read logs: %s", tx.Error)
		return
	}
	if err := templates["index.gohtml"].Execute(w, requests); err != nil {
		util.HTTPError("traffic", w, req, 500, "failed to read template: %s", err)
		return
	}
}
