package server

import (
	"net/http"

	"monks.co/apps/dungeon/db"
)

type Server struct {
	db  *db.DB
	hub *Hub
}

func New(d *db.DB) *Server {
	return &Server{db: d, hub: NewHub()}
}

func (s *Server) Mux() *http.ServeMux {
	mux := http.NewServeMux()

	// Static assets
	mux.HandleFunc("GET /static/index.js", s.handleStaticJS)
	mux.HandleFunc("GET /static/index.css", s.handleStaticCSS)

	// Pages
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /maps/{$}", s.handleCreateMap)
	mux.HandleFunc("GET /maps/{id}/{$}", s.handleMapView)

	// API
	mux.HandleFunc("GET /maps/{id}/state/{$}", s.handleMapState)
	mux.HandleFunc("GET /maps/{id}/events/{$}", s.handleSSE)
	mux.HandleFunc("POST /maps/{id}/cells/{$}", s.handleUpsertCells)
	mux.HandleFunc("POST /maps/{id}/walls/{$}", s.handleUpsertWall)
	mux.HandleFunc("POST /maps/{id}/markers/{$}", s.handleUpsertMarker)
	mux.HandleFunc("POST /maps/{id}/markers/delete/{$}", s.handleDeleteMarker)
	mux.HandleFunc("POST /maps/{id}/cells/delete/{$}", s.handleDeleteCells)

	return mux
}
