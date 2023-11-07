package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/a-h/templ"
	"monks.co/apps/posts/model"
	"monks.co/apps/posts/templates"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
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

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "" || req.URL.Path == "/" {
			h := templ.Handler(templates.Index(posts))
			h.ServeHTTP(w, req)
			return
		}
		slug := strings.TrimPrefix(req.URL.Path, "/")
		post := posts.Get(slug)
		h := templ.Handler(templates.Post(post))
		h.ServeHTTP(w, req)
	})

	ctx := sigctx.New()

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	if err := serve.ListenAndServe(ctx, addr, mux); err != nil {
		return err
	}
	return nil
}
