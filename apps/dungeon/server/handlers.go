package server

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"monks.co/apps/dungeon/db"
)

//go:generate go tool templ generate .

//go:embed static/index.js
var indexJS string

//go:embed static/index.css
var indexCSS string

func (s *Server) handleStaticJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	http.ServeContent(w, r, "index.js", time.Time{}, strings.NewReader(indexJS))
}

func (s *Server) handleStaticCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	http.ServeContent(w, r, "index.css", time.Time{}, strings.NewReader(indexCSS))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	maps, err := s.db.ListMaps()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	IndexPage(maps).Render(r.Context(), w)
}

func (s *Server) handleCreateMap(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	mapType := r.FormValue("type")
	if name == "" || (mapType != "dungeon" && mapType != "hex") {
		http.Error(w, "name and valid type required", http.StatusBadRequest)
		return
	}
	m := &db.Map{Name: name, Type: mapType}
	if err := s.db.CreateMap(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("../maps/%d/", m.ID), http.StatusSeeOther)
}

func (s *Server) handleMapView(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}
	m, err := s.db.GetMap(id)
	if err != nil {
		http.Error(w, "map not found", http.StatusNotFound)
		return
	}
	MapPage(*m).Render(r.Context(), w)
}

func (s *Server) handleMapState(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}
	state, err := s.db.GetMapState(id)
	if err != nil {
		http.Error(w, "map not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.hub.Subscribe(id)
	defer s.hub.Unsubscribe(id, ch)

	// Send initial ping
	fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) handleUpsertCells(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}

	var cells []db.Cell
	if err := json.NewDecoder(r.Body).Decode(&cells); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	for i := range cells {
		cells[i].MapID = id
	}
	if err := s.db.UpsertCells(cells); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Re-fetch the cells to get their IDs
	updated := make([]db.Cell, len(cells))
	for i, c := range cells {
		got, err := s.db.GetCell(id, c.X, c.Y)
		if err != nil {
			updated[i] = c
		} else {
			updated[i] = *got
		}
	}

	s.hub.Publish(id, "cells", updated)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleUpsertWall(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}

	var wall db.Wall
	if err := json.NewDecoder(r.Body).Decode(&wall); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	wall.MapID = id
	if err := s.db.UpsertWall(&wall); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.hub.Publish(id, "wall", wall)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wall)
}

func (s *Server) handleUpsertMarker(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}

	var marker db.Marker
	if err := json.NewDecoder(r.Body).Decode(&marker); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	marker.MapID = id
	if err := s.db.UpsertMarker(&marker); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.hub.Publish(id, "marker", marker)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(marker)
}

func (s *Server) handleDeleteMarker(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}

	var req struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := s.db.DeleteMarker(id, req.X, req.Y); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.hub.Publish(id, "marker_delete", req)

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteCells(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}

	var coords []struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	if err := json.NewDecoder(r.Body).Decode(&coords); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	pairs := make([][2]int, len(coords))
	for i, c := range coords {
		pairs[i] = [2]int{c.X, c.Y}
	}
	orphanedWalls, err := s.db.DeleteCellsAndCleanWalls(id, pairs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.hub.Publish(id, "cells_delete", coords)
	if len(orphanedWalls) > 0 {
		s.hub.Publish(id, "walls_delete", orphanedWalls)
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseID(r *http.Request, name string) (uint, error) {
	s := r.PathValue(name)
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(n), nil
}
