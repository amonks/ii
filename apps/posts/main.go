package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/a-h/templ"
	"monks.co/apps/posts/model"
	"monks.co/apps/posts/templates"
	"monks.co/pkg/gzip"
	"monks.co/pkg/ports"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

func main() {
	if err := run(); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx := sigctx.New()

	posts, err := model.LoadPosts("apps/posts/posts")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, req *http.Request) {
		h := templ.Handler(templates.Index(posts))
		h.ServeHTTP(w, req)
	})
	mux.HandleFunc("/{slug}", func(w http.ResponseWriter, req *http.Request) {
		slug := req.PathValue("slug")
		post := posts.Get(slug)
		if post == nil {
			serve.Errorf(w, req, http.StatusNotFound, "post '%s' not found", slug)
			return
		}
		component := templates.Post(post)
		h := templ.Handler(component)
		h.ServeHTTP(w, req)
	})

	port := ports.Apps["posts"]
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}
	return nil
}
