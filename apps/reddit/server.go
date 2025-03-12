package main

import (
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"monks.co/pkg/serve"
)

type Server struct {
	*serve.Mux
	db *model
}

func newServer(db *model) *Server {
	s := &Server{serve.NewMux(), db}

	s.Handle("GET /{$}", s.pageServer())

	fs := http.FileServer(http.Dir(archivePath))
	s.Handle("GET /media/", http.StripPrefix("/media/", fs))

	return s
}

type PageData struct {
	Posts []*Post
	Next  int
	Prev  int
}

func (s *Server) pageServer() http.Handler {
	const postsPerPage = 2
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		n := req.URL.Query().Get("n")
		if n == "" {
			n = "1"
		}
		offset, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			serve.Error(w, req, http.StatusBadRequest, err)
		}
		posts, err := s.db.getPosts(postsPerPage, int(offset))
		if err != nil {
			panic(err)
		}
		h := templ.Handler(Index(&PageData{
			Posts: posts,
			Next:  int(offset) + postsPerPage,
			Prev:  int(offset) - postsPerPage,
		}))
		h.ServeHTTP(w, req)
	})
}
