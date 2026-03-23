package node

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	pb "monks.co/apps/breadcrumbs/proto"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := OpenStore(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStoreOpen(t *testing.T) {
	s := testStore(t)
	// Verify tables exist by querying them.
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM points").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT count(*) FROM points_idx").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT count(*) FROM meta").Scan(&count); err != nil {
		t.Fatal(err)
	}
}

func TestStoreInsertAndQuery(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	p := &pb.Point{
		Timestamp:          1000,
		Latitude:           41.8781,
		Longitude:          -87.6298,
		Altitude:           180.0,
		HorizontalAccuracy: 5.0,
		Speed:              1.5,
		Course:             90.0,
		IsSimulated:        true,
	}
	if err := s.InsertPoint(ctx, p, math.MaxFloat64, false); err != nil {
		t.Fatal(err)
	}

	// Query a bbox that contains the point.
	pts, err := s.QueryTile(ctx, 41.0, 42.0, -88.0, -87.0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 1 {
		t.Fatalf("got %d points, want 1", len(pts))
	}
	got := pts[0]
	if got.Timestamp != 1000 {
		t.Errorf("Timestamp = %d", got.Timestamp)
	}
	if got.Latitude != 41.8781 {
		t.Errorf("Latitude = %f", got.Latitude)
	}
	if got.Longitude != -87.6298 {
		t.Errorf("Longitude = %f", got.Longitude)
	}
	if got.Altitude != 180.0 {
		t.Errorf("Altitude = %f", got.Altitude)
	}
	if !got.IsSimulated {
		t.Error("IsSimulated should be true")
	}
}

func TestStoreSpatialFilter(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Chicago
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1, Latitude: 41.8781, Longitude: -87.6298}, math.MaxFloat64, false)
	// New York
	s.InsertPoint(ctx, &pb.Point{Timestamp: 2, Latitude: 40.7128, Longitude: -74.0060}, math.MaxFloat64, false)
	// Tokyo
	s.InsertPoint(ctx, &pb.Point{Timestamp: 3, Latitude: 35.6762, Longitude: 139.6503}, math.MaxFloat64, false)

	// Query just Chicago area.
	pts, err := s.QueryTile(ctx, 41.0, 42.5, -88.5, -87.0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 1 {
		t.Fatalf("got %d points, want 1 (Chicago)", len(pts))
	}
	if pts[0].Timestamp != 1 {
		t.Errorf("got timestamp %d, want 1", pts[0].Timestamp)
	}
}

func TestStoreSignificanceFilter(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// High significance point.
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0}, 1.0, false)
	// Low significance point.
	s.InsertPoint(ctx, &pb.Point{Timestamp: 2, Latitude: 0.001, Longitude: 0.001}, 0.001, false)

	// Query with high threshold: only the high-sig point.
	pts, err := s.QueryTile(ctx, -1, 1, -1, 1, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 1 {
		t.Fatalf("high threshold: got %d points, want 1", len(pts))
	}

	// Query with low threshold: both points.
	pts, err = s.QueryTile(ctx, -1, 1, -1, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 2 {
		t.Fatalf("low threshold: got %d points, want 2", len(pts))
	}
}

func TestStoreDedup(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	p1 := &pb.Point{Timestamp: 100, Latitude: 1.0, Longitude: 2.0}
	if err := s.InsertPoint(ctx, p1, 0.5, false); err != nil {
		t.Fatal(err)
	}

	// Insert again with same timestamp, different coords.
	p2 := &pb.Point{Timestamp: 100, Latitude: 3.0, Longitude: 4.0}
	if err := s.InsertPoint(ctx, p2, 0.5, false); err != nil {
		t.Fatal(err)
	}

	// Should have exactly one point with updated coords.
	pts, err := s.QueryTile(ctx, -90, 90, -180, 180, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 1 {
		t.Fatalf("got %d points after dedup, want 1", len(pts))
	}
	if pts[0].Latitude != 3.0 {
		t.Errorf("Latitude = %f, want 3.0 (updated)", pts[0].Latitude)
	}
}

func TestStoreUpdateSignificance(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.InsertPoint(ctx, &pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0}, math.MaxFloat64, false)

	// Update significance to a low value.
	if err := s.UpdateSignificance(ctx, 1, 0.001); err != nil {
		t.Fatal(err)
	}

	// With high threshold, point should be excluded.
	pts, err := s.QueryTile(ctx, -1, 1, -1, 1, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 0 {
		t.Fatalf("got %d points, want 0 after significance update", len(pts))
	}
}

func TestStoreStatsEmpty(t *testing.T) {
	s := testStore(t)
	count, latest, err := s.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
	if latest != nil {
		t.Errorf("latest = %v, want nil", latest)
	}
}

func TestStoreStats(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1000, Latitude: 10, Longitude: 20}, math.MaxFloat64, false)
	s.InsertPoint(ctx, &pb.Point{Timestamp: 3000, Latitude: 30, Longitude: 40}, math.MaxFloat64, false)
	s.InsertPoint(ctx, &pb.Point{Timestamp: 2000, Latitude: 50, Longitude: 60}, math.MaxFloat64, false)

	count, latest, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
	if latest == nil {
		t.Fatal("latest is nil")
	}
	if latest.Timestamp != 3000 {
		t.Errorf("latest.Timestamp = %d, want 3000", latest.Timestamp)
	}
	if latest.Latitude != 30 {
		t.Errorf("latest.Latitude = %f, want 30", latest.Latitude)
	}
}

func TestNextVisiblePoint(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1000, Latitude: 10, Longitude: 20}, 0.5, false)
	s.InsertPoint(ctx, &pb.Point{Timestamp: 2000, Latitude: 11, Longitude: 21}, 0.01, false)
	s.InsertPoint(ctx, &pb.Point{Timestamp: 3000, Latitude: 12, Longitude: 22}, 0.5, false)

	// Should skip ts=2000 (sig 0.01 < 0.1) and return ts=3000.
	p, err := s.NextVisiblePoint(ctx, 1000, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Timestamp != 3000 {
		t.Errorf("got %v, want timestamp 3000", p)
	}
}

func TestNextVisiblePointNone(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1000, Latitude: 10, Longitude: 20}, 0.5, false)

	p, err := s.NextVisiblePoint(ctx, 1000, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Errorf("got %v, want nil", p)
	}
}

func TestPrevVisiblePoint(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1000, Latitude: 10, Longitude: 20}, 0.5, false)
	s.InsertPoint(ctx, &pb.Point{Timestamp: 2000, Latitude: 11, Longitude: 21}, 0.01, false)
	s.InsertPoint(ctx, &pb.Point{Timestamp: 3000, Latitude: 12, Longitude: 22}, 0.5, false)

	// Should skip ts=2000 (sig 0.01 < 0.1) and return ts=1000.
	p, err := s.PrevVisiblePoint(ctx, 3000, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Timestamp != 1000 {
		t.Errorf("got %v, want timestamp 1000", p)
	}
}

func TestPrevVisiblePointNone(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1000, Latitude: 10, Longitude: 20}, 0.5, false)

	p, err := s.PrevVisiblePoint(ctx, 1000, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Errorf("got %v, want nil", p)
	}
}

func TestGlobalTimestampRange(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Empty store.
	first, last, err := s.GlobalTimestampRange(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first != 0 || last != 0 {
		t.Errorf("empty: got (%d, %d), want (0, 0)", first, last)
	}

	s.InsertPoint(ctx, &pb.Point{Timestamp: 3000, Latitude: 10, Longitude: 20}, 0.5, false)
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1000, Latitude: 11, Longitude: 21}, 0.5, false)
	s.InsertPoint(ctx, &pb.Point{Timestamp: 5000, Latitude: 12, Longitude: 22}, 0.5, false)

	first, last, err = s.GlobalTimestampRange(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first != 1000 || last != 5000 {
		t.Errorf("got (%d, %d), want (1000, 5000)", first, last)
	}
}

func TestStoreLastTwoPointsEmpty(t *testing.T) {
	s := testStore(t)
	prev, tail, err := s.LastTwoPoints(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if prev != nil || tail != nil {
		t.Errorf("expected nil, nil; got %v, %v", prev, tail)
	}
}

func TestStoreLastTwoPointsOne(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1, Latitude: 10, Longitude: 20}, math.MaxFloat64, false)

	prev, tail, err := s.LastTwoPoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if prev != nil {
		t.Errorf("prev should be nil, got %v", prev)
	}
	if tail == nil || tail.Timestamp != 1 {
		t.Errorf("tail = %v, want timestamp 1", tail)
	}
}

func TestStoreLastTwoPoints(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1, Latitude: 10, Longitude: 20}, math.MaxFloat64, false)
	s.InsertPoint(ctx, &pb.Point{Timestamp: 2, Latitude: 11, Longitude: 21}, math.MaxFloat64, false)
	s.InsertPoint(ctx, &pb.Point{Timestamp: 3, Latitude: 12, Longitude: 22}, 0.5, false)

	prev, tail, err := s.LastTwoPoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if prev == nil || prev.Timestamp != 2 {
		t.Errorf("prev = %v, want timestamp 2", prev)
	}
	if tail == nil || tail.Timestamp != 3 {
		t.Errorf("tail = %v, want timestamp 3", tail)
	}
}
