package db

import (
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	return db
}

func TestMapRoundTrip(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Test Dungeon", Type: "dungeon"}
	if err := db.CreateMap(m); err != nil {
		t.Fatalf("CreateMap: %v", err)
	}
	if m.ID == 0 {
		t.Fatal("expected ID to be set after create")
	}
	if m.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}

	got, err := db.GetMap(m.ID)
	if err != nil {
		t.Fatalf("GetMap: %v", err)
	}
	if got.Name != "Test Dungeon" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Dungeon")
	}
	if got.Type != "dungeon" {
		t.Errorf("Type = %q, want %q", got.Type, "dungeon")
	}
}

func TestListMaps(t *testing.T) {
	db := newTestDB(t)

	db.CreateMap(&Map{Name: "Map A", Type: "hex"})
	db.CreateMap(&Map{Name: "Map B", Type: "dungeon"})

	maps, err := db.ListMaps()
	if err != nil {
		t.Fatalf("ListMaps: %v", err)
	}
	if len(maps) != 2 {
		t.Fatalf("got %d maps, want 2", len(maps))
	}
	// Most recent first
	if maps[0].Name != "Map B" {
		t.Errorf("first map = %q, want %q", maps[0].Name, "Map B")
	}
}

func TestDeleteMap(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Doomed", Type: "dungeon"}
	db.CreateMap(m)
	db.UpsertCell(&Cell{MapID: m.ID, X: 0, Y: 0, IsExplored: true})
	db.UpsertWall(&Wall{MapID: m.ID, X1: 0, Y1: 0, X2: 1, Y2: 0, Type: "open"})
	db.UpsertMarker(&Marker{MapID: m.ID, X: 0, Y: 0, Letter: "A"})

	if err := db.DeleteMap(m.ID); err != nil {
		t.Fatalf("DeleteMap: %v", err)
	}

	_, err := db.GetMap(m.ID)
	if err == nil {
		t.Fatal("expected error getting deleted map")
	}
	cells, _ := db.ListCells(m.ID)
	if len(cells) != 0 {
		t.Errorf("expected 0 cells after delete, got %d", len(cells))
	}
	walls, _ := db.ListWalls(m.ID)
	if len(walls) != 0 {
		t.Errorf("expected 0 walls after delete, got %d", len(walls))
	}
	markers, _ := db.ListMarkers(m.ID)
	if len(markers) != 0 {
		t.Errorf("expected 0 markers after delete, got %d", len(markers))
	}
}

func TestCellUpsert(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Test", Type: "dungeon"}
	db.CreateMap(m)

	// Create cell
	c := &Cell{MapID: m.ID, X: 3, Y: 5, IsExplored: false}
	if err := db.UpsertCell(c); err != nil {
		t.Fatalf("UpsertCell (create): %v", err)
	}
	if c.ID == 0 {
		t.Fatal("expected cell ID to be set")
	}

	// Update same cell
	c2 := &Cell{MapID: m.ID, X: 3, Y: 5, IsExplored: true, Text: "treasure"}
	if err := db.UpsertCell(c2); err != nil {
		t.Fatalf("UpsertCell (update): %v", err)
	}
	if c2.ID != c.ID {
		t.Errorf("expected same ID on upsert, got %d vs %d", c2.ID, c.ID)
	}

	got, err := db.GetCell(m.ID, 3, 5)
	if err != nil {
		t.Fatalf("GetCell: %v", err)
	}
	if !got.IsExplored {
		t.Error("expected IsExplored=true")
	}
	if got.Text != "treasure" {
		t.Errorf("Text = %q, want %q", got.Text, "treasure")
	}
}

func TestCellHue(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Test", Type: "dungeon"}
	db.CreateMap(m)

	hue := 120
	c := &Cell{MapID: m.ID, X: 0, Y: 0, Hue: &hue}
	db.UpsertCell(c)

	got, _ := db.GetCell(m.ID, 0, 0)
	if got.Hue == nil || *got.Hue != 120 {
		t.Errorf("Hue = %v, want 120", got.Hue)
	}
}

func TestUpsertCells(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Test", Type: "dungeon"}
	db.CreateMap(m)

	roomID := uint(1)
	cells := []Cell{
		{MapID: m.ID, X: 0, Y: 0, RoomID: &roomID},
		{MapID: m.ID, X: 1, Y: 0, RoomID: &roomID},
		{MapID: m.ID, X: 0, Y: 1, RoomID: &roomID},
	}
	if err := db.UpsertCells(cells); err != nil {
		t.Fatalf("UpsertCells: %v", err)
	}

	got, _ := db.ListCells(m.ID)
	if len(got) != 3 {
		t.Fatalf("got %d cells, want 3", len(got))
	}
}

func TestNextRoomID(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Test", Type: "dungeon"}
	db.CreateMap(m)

	id, err := db.NextRoomID(m.ID)
	if err != nil {
		t.Fatalf("NextRoomID: %v", err)
	}
	if id != 1 {
		t.Errorf("first room ID = %d, want 1", id)
	}

	roomID := uint(1)
	db.UpsertCell(&Cell{MapID: m.ID, X: 0, Y: 0, RoomID: &roomID})

	id, _ = db.NextRoomID(m.ID)
	if id != 2 {
		t.Errorf("second room ID = %d, want 2", id)
	}
}

func TestWallUpsert(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Test", Type: "dungeon"}
	db.CreateMap(m)

	w := &Wall{MapID: m.ID, X1: 0, Y1: 0, X2: 1, Y2: 0, Type: "open"}
	if err := db.UpsertWall(w); err != nil {
		t.Fatalf("UpsertWall: %v", err)
	}

	// Update same wall
	w2 := &Wall{MapID: m.ID, X1: 0, Y1: 0, X2: 1, Y2: 0, Type: "door"}
	if err := db.UpsertWall(w2); err != nil {
		t.Fatalf("UpsertWall (update): %v", err)
	}

	walls, _ := db.ListWalls(m.ID)
	if len(walls) != 1 {
		t.Fatalf("got %d walls, want 1", len(walls))
	}
	if walls[0].Type != "door" {
		t.Errorf("wall type = %q, want %q", walls[0].Type, "door")
	}
}

func TestWallNormalization(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Test", Type: "dungeon"}
	db.CreateMap(m)

	// Insert with reversed coordinates
	w := &Wall{MapID: m.ID, X1: 1, Y1: 0, X2: 0, Y2: 0, Type: "open"}
	db.UpsertWall(w)

	walls, _ := db.ListWalls(m.ID)
	if len(walls) != 1 {
		t.Fatalf("got %d walls, want 1", len(walls))
	}
	// Should be normalized to (0,0)-(1,0)
	if walls[0].X1 != 0 || walls[0].Y1 != 0 || walls[0].X2 != 1 || walls[0].Y2 != 0 {
		t.Errorf("wall not normalized: (%d,%d)-(%d,%d)", walls[0].X1, walls[0].Y1, walls[0].X2, walls[0].Y2)
	}
}

func TestDeleteWall(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Test", Type: "dungeon"}
	db.CreateMap(m)

	db.UpsertWall(&Wall{MapID: m.ID, X1: 0, Y1: 0, X2: 1, Y2: 0, Type: "open"})

	if err := db.DeleteWall(m.ID, 0, 0, 1, 0); err != nil {
		t.Fatalf("DeleteWall: %v", err)
	}

	walls, _ := db.ListWalls(m.ID)
	if len(walls) != 0 {
		t.Errorf("got %d walls after delete, want 0", len(walls))
	}
}

func TestMarkerUpsert(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Test", Type: "dungeon"}
	db.CreateMap(m)

	mk := &Marker{MapID: m.ID, X: 2, Y: 3, Letter: "A"}
	if err := db.UpsertMarker(mk); err != nil {
		t.Fatalf("UpsertMarker: %v", err)
	}

	// Update same marker position
	mk2 := &Marker{MapID: m.ID, X: 2, Y: 3, Letter: "B"}
	db.UpsertMarker(mk2)

	markers, _ := db.ListMarkers(m.ID)
	if len(markers) != 1 {
		t.Fatalf("got %d markers, want 1", len(markers))
	}
	if markers[0].Letter != "B" {
		t.Errorf("marker letter = %q, want %q", markers[0].Letter, "B")
	}
}

func TestDeleteMarker(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Test", Type: "dungeon"}
	db.CreateMap(m)

	db.UpsertMarker(&Marker{MapID: m.ID, X: 1, Y: 1, Letter: "S"})

	if err := db.DeleteMarker(m.ID, 1, 1); err != nil {
		t.Fatalf("DeleteMarker: %v", err)
	}

	markers, _ := db.ListMarkers(m.ID)
	if len(markers) != 0 {
		t.Errorf("got %d markers after delete, want 0", len(markers))
	}
}

func TestMapState(t *testing.T) {
	db := newTestDB(t)

	m := &Map{Name: "Full Map", Type: "dungeon"}
	db.CreateMap(m)

	roomID := uint(1)
	db.UpsertCell(&Cell{MapID: m.ID, X: 0, Y: 0, IsExplored: true, RoomID: &roomID})
	db.UpsertCell(&Cell{MapID: m.ID, X: 1, Y: 0, IsExplored: true, RoomID: &roomID})
	db.UpsertWall(&Wall{MapID: m.ID, X1: 0, Y1: 0, X2: 1, Y2: 0, Type: "open"})
	db.UpsertMarker(&Marker{MapID: m.ID, X: 0, Y: 0, Letter: "A"})

	state, err := db.GetMapState(m.ID)
	if err != nil {
		t.Fatalf("GetMapState: %v", err)
	}
	if state.Map.Name != "Full Map" {
		t.Errorf("map name = %q, want %q", state.Map.Name, "Full Map")
	}
	if len(state.Cells) != 2 {
		t.Errorf("got %d cells, want 2", len(state.Cells))
	}
	if len(state.Walls) != 1 {
		t.Errorf("got %d walls, want 1", len(state.Walls))
	}
	if len(state.Markers) != 1 {
		t.Errorf("got %d markers, want 1", len(state.Markers))
	}
}
