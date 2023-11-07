package main

import (
	"html/template"
	"net/http"
	"strconv"

	"monks.co/pkg/serve"
)

type Server struct {
	*http.ServeMux
	db *model
}

func newServer(db *model) *Server {
	s := &Server{http.NewServeMux(), db}

	s.Handle("/", s.pageServer())

	fs := http.FileServer(http.Dir(archivePath))
	s.Handle("/media/", http.StripPrefix("/media/", fs))

	return s
}

func (s *Server) pageServer() http.Handler {
	tmpl := template.Must(template.ParseFiles("index.gohtml"))
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		n := req.URL.Query().Get("n")
		if n == "" {
			n = "1"
		}
		offset, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			serve.Error(res, req, http.StatusBadRequest, err)
		}
		posts, err := s.db.getPosts(1, int(offset))
		if err != nil {
			panic(err)
		}
		tmpl.Execute(res, struct {
			Posts []*Post
			Next  int
		}{posts, int(offset) + 1})
	})
}
