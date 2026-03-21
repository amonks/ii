package node

import (
	"testing"

	pb "monks.co/apps/breadcrumbs/proto"
	"google.golang.org/protobuf/proto"
)

func TestEncodeMVTEmpty(t *testing.T) {
	data, err := EncodeMVT(nil, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	var tile pb.Tile
	if err := proto.Unmarshal(data, &tile); err != nil {
		t.Fatal(err)
	}
	if len(tile.Layers) != 1 {
		t.Fatalf("layers = %d, want 1", len(tile.Layers))
	}
	if tile.Layers[0].GetName() != "track" {
		t.Errorf("layer name = %q", tile.Layers[0].GetName())
	}
	if tile.Layers[0].GetExtent() != 4096 {
		t.Errorf("extent = %d", tile.Layers[0].GetExtent())
	}
}

func TestEncodeMVTSinglePoint(t *testing.T) {
	points := []*pb.Point{
		{Latitude: 0, Longitude: 0},
	}
	data, err := EncodeMVT(points, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	var tile pb.Tile
	if err := proto.Unmarshal(data, &tile); err != nil {
		t.Fatal(err)
	}

	feat := tile.Layers[0].Features[0]
	if feat.GetType() != pb.Tile_LINESTRING {
		t.Errorf("type = %v, want LINESTRING", feat.GetType())
	}
	// MoveTo(1) + dx + dy = 3 commands
	if len(feat.Geometry) != 3 {
		t.Errorf("geometry len = %d, want 3", len(feat.Geometry))
	}
}

func TestEncodeMVTLineString(t *testing.T) {
	// Three points in a line within tile (0,0,0).
	points := []*pb.Point{
		{Latitude: 0, Longitude: -180},  // left edge, equator
		{Latitude: 0, Longitude: 0},     // center, equator
		{Latitude: 0, Longitude: 180},   // right edge, equator
	}
	data, err := EncodeMVT(points, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	var tile pb.Tile
	if err := proto.Unmarshal(data, &tile); err != nil {
		t.Fatal(err)
	}

	feat := tile.Layers[0].Features[0]
	// MoveTo(1) + 2 coords + LineTo(2) + 4 coords = 8
	if len(feat.Geometry) != 8 {
		t.Errorf("geometry len = %d, want 8", len(feat.Geometry))
	}
}

func TestEncodeMVTMercatorProjection(t *testing.T) {
	// At zoom 0, the equator (lat=0) should map to the vertical midpoint
	// of the tile in Mercator projection (tileY = 2048 for extent 4096).
	points := []*pb.Point{
		{Latitude: 0, Longitude: 0},
	}
	data, err := EncodeMVT(points, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	var tile pb.Tile
	if err := proto.Unmarshal(data, &tile); err != nil {
		t.Fatal(err)
	}

	feat := tile.Layers[0].Features[0]
	// Geometry: MoveTo(1) + zigzag(dx) + zigzag(dy) = 3 values
	if len(feat.Geometry) < 3 {
		t.Fatalf("geometry len = %d, want >= 3", len(feat.Geometry))
	}

	// Decode: dx and dy are zigzag-encoded.
	dx := decodeZigzag(feat.Geometry[1])
	dy := decodeZigzag(feat.Geometry[2])

	// Equator, lon=0 → center of tile (0,0,0), i.e. tileX=2048, tileY=2048.
	if dx != 2048 {
		t.Errorf("tileX = %d, want 2048 (center)", dx)
	}
	if dy != 2048 {
		t.Errorf("tileY = %d, want 2048 (equator in Mercator)", dy)
	}
}

func decodeZigzag(v uint32) int {
	return int(int32(v>>1) ^ -int32(v&1))
}

func TestZigzag(t *testing.T) {
	tests := []struct {
		in  int
		out uint32
	}{
		{0, 0},
		{-1, 1},
		{1, 2},
		{-2, 3},
		{2, 4},
	}
	for _, tt := range tests {
		got := zigzag(tt.in)
		if got != tt.out {
			t.Errorf("zigzag(%d) = %d, want %d", tt.in, got, tt.out)
		}
	}
}
