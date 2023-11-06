package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/a-h/templ"
	"monks.co/apps/youtube/model"
	"monks.co/apps/youtube/templates"
	"monks.co/pkg/gzip"
)

var port = flag.Int("port", 3000, "port")

func main() {
	if err := run(); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	history, err := model.LoadHistory("histories")
	if err != nil {
		return err
	}

	http.Handle("/", gzip.Middleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		h := templ.Handler(templates.Index(history))
		w.Header().Set("Content-type", "charset=utf-8")
		h.ServeHTTP(w, req)
		return
	})))

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	fmt.Println("listening on", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		return err
	}
	return nil
}
