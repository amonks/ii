package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"monks.co/apps/dungeon/db"
)

func newTestServer(t *testing.T) (*Server, *db.DB) {
	t.Helper()
	d, err := db.NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	s := New(d)
	return s, d
}

func TestHandleIndex(t *testing.T) {
	s, _ := newTestServer(t)
	mux := s.Mux()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "Dungeon Maps") {
		t.Error("expected index page to contain 'Dungeon Maps'")
	}
}

func TestHandleCreateMap(t *testing.T) {
	s, d := newTestServer(t)
	mux := s.Mux()

	form := url.Values{"name": {"Test Map"}, "type": {"dungeon"}}
	req := httptest.NewRequest("POST", "/maps/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}

	maps, _ := d.ListMaps()
	if len(maps) != 1 {
		t.Fatalf("expected 1 map, got %d", len(maps))
	}
	if maps[0].Name != "Test Map" {
		t.Errorf("map name = %q, want %q", maps[0].Name, "Test Map")
	}
}

func TestHandleMapState(t *testing.T) {
	s, d := newTestServer(t)
	mux := s.Mux()

	m := &db.Map{Name: "Test", Type: "dungeon"}
	d.CreateMap(m)
	d.UpsertCell(&db.Cell{MapID: m.ID, X: 0, Y: 0, IsExplored: true})

	req := httptest.NewRequest("GET", "/maps/1/state/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var state db.MapState
	if err := json.NewDecoder(w.Body).Decode(&state); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if state.Map.Name != "Test" {
		t.Errorf("map name = %q, want %q", state.Map.Name, "Test")
	}
	if len(state.Cells) != 1 {
		t.Errorf("got %d cells, want 1", len(state.Cells))
	}
}

func TestHandleUpsertCells(t *testing.T) {
	s, d := newTestServer(t)
	mux := s.Mux()

	m := &db.Map{Name: "Test", Type: "dungeon"}
	d.CreateMap(m)

	roomID := uint(1)
	cells := []db.Cell{
		{X: 0, Y: 0, IsExplored: true, RoomID: &roomID},
		{X: 1, Y: 0, IsExplored: true, RoomID: &roomID},
	}
	body, _ := json.Marshal(cells)
	req := httptest.NewRequest("POST", "/maps/1/cells/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	got, _ := d.ListCells(m.ID)
	if len(got) != 2 {
		t.Errorf("got %d cells, want 2", len(got))
	}
}

func TestHandleUpsertWall(t *testing.T) {
	s, d := newTestServer(t)
	mux := s.Mux()

	m := &db.Map{Name: "Test", Type: "dungeon"}
	d.CreateMap(m)

	wall := db.Wall{X1: 0, Y1: 0, X2: 1, Y2: 0, Type: "open"}
	body, _ := json.Marshal(wall)
	req := httptest.NewRequest("POST", "/maps/1/walls/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	walls, _ := d.ListWalls(m.ID)
	if len(walls) != 1 {
		t.Errorf("got %d walls, want 1", len(walls))
	}
}

func TestHandleMarkers(t *testing.T) {
	s, d := newTestServer(t)
	mux := s.Mux()

	m := &db.Map{Name: "Test", Type: "dungeon"}
	d.CreateMap(m)

	// Create marker
	marker := db.Marker{X: 2, Y: 3, Letter: "S"}
	body, _ := json.Marshal(marker)
	req := httptest.NewRequest("POST", "/maps/1/markers/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("create: status = %d, want %d", w.Code, http.StatusOK)
	}

	// Delete marker
	delBody, _ := json.Marshal(map[string]int{"x": 2, "y": 3})
	req = httptest.NewRequest("POST", "/maps/1/markers/delete/", bytes.NewReader(delBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("delete: status = %d, want %d", w.Code, http.StatusNoContent)
	}

	markers, _ := d.ListMarkers(m.ID)
	if len(markers) != 0 {
		t.Errorf("got %d markers after delete, want 0", len(markers))
	}
}
