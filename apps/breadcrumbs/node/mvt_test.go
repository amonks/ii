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
	// A single point can't form a LINESTRING, so the tile should have no features.
	data, err := EncodeMVT([][]*pb.Point{
		{{Latitude: 0, Longitude: 0}},
	}, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	var tile pb.Tile
	if err := proto.Unmarshal(data, &tile); err != nil {
		t.Fatal(err)
	}

	if len(tile.Layers[0].Features) != 0 {
		t.Errorf("features = %d, want 0 for single point", len(tile.Layers[0].Features))
	}
}

func TestEncodeMVTLineString(t *testing.T) {
	// Three points in a line within tile (0,0,0).
	data, err := EncodeMVT([][]*pb.Point{{
		{Latitude: 0, Longitude: -180},
		{Latitude: 0, Longitude: 0},
		{Latitude: 0, Longitude: 180},
	}}, 0, 0, 0)
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
	data, err := EncodeMVT([][]*pb.Point{{
		{Latitude: 0, Longitude: 0},
		{Latitude: 0, Longitude: 1},
	}}, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	var tile pb.Tile
	if err := proto.Unmarshal(data, &tile); err != nil {
		t.Fatal(err)
	}

	feat := tile.Layers[0].Features[0]
	if len(feat.Geometry) < 3 {
		t.Fatalf("geometry len = %d, want >= 3", len(feat.Geometry))
	}

	dx := decodeZigzag(feat.Geometry[1])
	dy := decodeZigzag(feat.Geometry[2])

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

func TestEncodeMVTDownsamplesLargeTile(t *testing.T) {
	n := maxPointsPerTile + 10000
	points := make([]*pb.Point, n)
	for i := range n {
		frac := float64(i) / float64(n-1)
		points[i] = &pb.Point{
			Latitude:  frac * 10,
			Longitude: -180 + frac*360,
		}
	}

	data, err := EncodeMVT([][]*pb.Point{points}, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	var tile pb.Tile
	if err := proto.Unmarshal(data, &tile); err != nil {
		t.Fatal(err)
	}

	features := tile.Layers[0].Features
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}

	verts := (len(features[0].Geometry)-4)/2 + 1
	if verts > maxPointsPerTile {
		t.Errorf("tile has %d vertices, exceeds limit %d", verts, maxPointsPerTile)
	}
	if verts < maxPointsPerTile-1 {
		t.Errorf("tile has %d vertices, expected close to %d", verts, maxPointsPerTile)
	}
}

func TestEncodeMVTMultipleSegments(t *testing.T) {
	// Two segments should produce two LineString features.
	data, err := EncodeMVT([][]*pb.Point{
		{
			{Latitude: 0, Longitude: -180},
			{Latitude: 0, Longitude: 0},
		},
		{
			{Latitude: 10, Longitude: 0},
			{Latitude: 10, Longitude: 180},
		},
	}, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	var tile pb.Tile
	if err := proto.Unmarshal(data, &tile); err != nil {
		t.Fatal(err)
	}

	if len(tile.Layers[0].Features) != 2 {
		t.Errorf("features = %d, want 2", len(tile.Layers[0].Features))
	}
}

func TestEncodeMVTSinglePointSegmentDropped(t *testing.T) {
	// A segment with 1 point can't form a line and should be dropped.
	data, err := EncodeMVT([][]*pb.Point{
		{{Latitude: 0, Longitude: 0}},
		{
			{Latitude: 10, Longitude: 0},
			{Latitude: 10, Longitude: 180},
		},
	}, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	var tile pb.Tile
	if err := proto.Unmarshal(data, &tile); err != nil {
		t.Fatal(err)
	}

	if len(tile.Layers[0].Features) != 1 {
		t.Errorf("features = %d, want 1 (single-point segment dropped)", len(tile.Layers[0].Features))
	}
}

func TestDownsamplePreservesFirstAndLast(t *testing.T) {
	pts := make([]*pb.Point, 100)
	for i := range pts {
		pts[i] = &pb.Point{Timestamp: int64(i)}
	}
	out := downsample(pts, 10)
	if len(out) != 10 {
		t.Fatalf("len = %d, want 10", len(out))
	}
	if out[0].Timestamp != 0 {
		t.Errorf("first = %d, want 0", out[0].Timestamp)
	}
	if out[9].Timestamp != 99 {
		t.Errorf("last = %d, want 99", out[9].Timestamp)
	}
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
