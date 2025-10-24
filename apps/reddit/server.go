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

	// New routes
	s.Handle("GET /{$}", s.listServer())                  // List page is now the home page
	s.Handle("GET /post/{n}/{$}", s.postServer())         // Individual post page with trailing slash
	s.Handle("GET /subreddits/{$}", s.subredditsServer()) // Subreddits page
	s.Handle("GET /authors/{$}", s.authorsServer())       // Authors page
	s.Handle("POST /star/{name}/{$}", s.starHandler())    // Star toggle endpoint

	fs := http.FileServer(http.Dir(archivePath))
	s.Handle("GET /media/", http.StripPrefix("/media/", fs))

	return s
}

type PageData struct {
	Post       *Post
	Next       int
	Prev       int
	Subreddit  string
	Author     string
	Starred    string // Starred filter parameter
	Current    int // Current position in the posts list
	TotalPosts int // Total number of posts
}

type ListData struct {
	Posts     []*Post
	Subreddit string
	Author    string
	Starred   string
}

func (s *Server) postServer() http.Handler {
	const postsPerPage = 1
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Get post number from path parameter
		n := req.PathValue("n")
		if n == "" {
			n = "1"
		}

		// Get filter parameters
		subreddit := req.URL.Query().Get("subreddit")
		author := req.URL.Query().Get("author")
		starredParam := req.URL.Query().Get("starred")
		
		var starred *bool
		if starredParam == "true" {
			starred = &[]bool{true}[0]
		} else if starredParam == "false" {
			starred = &[]bool{false}[0]
		}

		// Get total count of posts for wrap-around navigation
		totalCount, err := s.db.getPostCount(subreddit, author, starred)
		if err != nil {
			serve.Error(w, req, http.StatusInternalServerError, err)
			return
		}

		if totalCount == 0 {
			serve.Error(w, req, http.StatusNotFound, nil)
			return
		}

		offset, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			serve.Error(w, req, http.StatusBadRequest, err)
			return
		}

		// Implement wrap-around for offset
		// First ensure offset is positive (may be negative if user tried to go back from 1)
		for offset <= 0 {
			offset += totalCount
		}

		// Then take modulo to ensure it's within bounds (1 to totalCount)
		offset = ((offset - 1) % totalCount) + 1

		posts, err := s.db.getPosts(postsPerPage, int(offset), subreddit, author, starred)
		if err != nil {
			serve.Error(w, req, http.StatusInternalServerError, err)
			return
		}

		if len(posts) == 0 {
			serve.Error(w, req, http.StatusNotFound, nil)
			return
		}

		// Calculate next and previous with wrap-around
		next := (offset % totalCount) + 1
		prev := offset - 1
		if prev == 0 {
			prev = totalCount
		}

		// Add the current position and total count to the page data
		h := templ.Handler(PostPage(&PageData{
			Post:       posts[0],
			Next:       int(next),
			Prev:       int(prev),
			Subreddit:  subreddit,
			Author:     author,
			Starred:    starredParam,
			Current:    int(offset),
			TotalPosts: int(totalCount),
		}))
		h.ServeHTTP(w, req)
	})
}

func (s *Server) listServer() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Get filter parameters
		subreddit := req.URL.Query().Get("subreddit")
		author := req.URL.Query().Get("author")
		starredParam := req.URL.Query().Get("starred")
		
		var starred *bool
		if starredParam == "true" {
			starred = &[]bool{true}[0]
		} else if starredParam == "false" {
			starred = &[]bool{false}[0]
		}

		posts, err := s.db.getPosts(1000, 1, subreddit, author, starred)
		if err != nil {
			serve.Error(w, req, http.StatusInternalServerError, err)
			return
		}

		h := templ.Handler(List(&ListData{
			Posts:     posts,
			Subreddit: subreddit,
			Author:    author,
			Starred:   starredParam,
		}))
		h.ServeHTTP(w, req)
	})
}

func (s *Server) subredditsServer() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		subreddits, err := s.db.getSubredditCounts()
		if err != nil {
			serve.Error(w, req, http.StatusInternalServerError, err)
			return
		}

		h := templ.Handler(Subreddits(&SubredditsData{
			Subreddits: subreddits,
		}))
		h.ServeHTTP(w, req)
	})
}

func (s *Server) authorsServer() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		authors, err := s.db.getAuthorCounts()
		if err != nil {
			serve.Error(w, req, http.StatusInternalServerError, err)
			return
		}

		h := templ.Handler(Authors(&AuthorsData{
			Authors: authors,
		}))
		h.ServeHTTP(w, req)
	})
}

func (s *Server) starHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		name := req.PathValue("name")
		if name == "" {
			serve.Error(w, req, http.StatusBadRequest, nil)
			return
		}

		if err := s.db.toggleStarredStatus(name); err != nil {
			serve.Error(w, req, http.StatusInternalServerError, err)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
