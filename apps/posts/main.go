package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/a-h/templ"
	"monks.co/apps/posts/model"
	"monks.co/apps/posts/templates"
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

	posts, err := model.LoadPosts("apps/posts/posts")
	if err != nil {
		return err
	}

	fmt.Println("title:", posts.List[0].Title)

	http.Handle("/", templ.Handler(templates.Index(posts)))

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	fmt.Println("listening on", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		return err
	}
	return nil
}
