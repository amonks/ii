package server

import (
	"net/http"

	"monks.co/apps/dolmenwood/db"
)

type Server struct {
	db *db.DB
}

func New(d *db.DB) *Server {
	return &Server{db: d}
}

func (s *Server) Mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /characters/{$}", s.handleCreateCharacter)
	mux.HandleFunc("GET /characters/{id}/{$}", s.handleCharacterSheet)
	mux.HandleFunc("POST /characters/{id}/hp/{$}", s.handleUpdateHP)
	mux.HandleFunc("POST /characters/{id}/items/{$}", s.handleAddItem)
	mux.HandleFunc("POST /characters/{id}/items/{itemID}/update/{$}", s.handleUpdateItem)
	mux.HandleFunc("POST /characters/{id}/items/{itemID}/split/{$}", s.handleSplitItem)
	mux.HandleFunc("POST /characters/{id}/items/{itemID}/decrement/{$}", s.handleDecrementItem)
	mux.HandleFunc("POST /characters/{id}/items/{itemID}/delete/{$}", s.handleDeleteItem)
	mux.HandleFunc("POST /characters/{id}/companions/{$}", s.handleAddCompanion)
	mux.HandleFunc("POST /characters/{id}/companions/{compID}/update/{$}", s.handleUpdateCompanion)
	mux.HandleFunc("POST /characters/{id}/companions/{compID}/delete/{$}", s.handleDeleteCompanion)
	mux.HandleFunc("POST /characters/{id}/treasure/{$}", s.handleAddTreasure)
	mux.HandleFunc("POST /characters/{id}/treasure/{txID}/undo/{$}", s.handleUndoTransaction)
	mux.HandleFunc("POST /characters/{id}/xp/{$}", s.handleAddXP)
	mux.HandleFunc("POST /characters/{id}/return-to-safety/{$}", s.handleReturnToSafety)
	mux.HandleFunc("POST /characters/{id}/level-up/{$}", s.handleLevelUp)
	mux.HandleFunc("POST /characters/{id}/notes/{$}", s.handleAddNote)
	mux.HandleFunc("POST /characters/{id}/notes/{noteID}/delete/{$}", s.handleDeleteNote)
	return mux
}
