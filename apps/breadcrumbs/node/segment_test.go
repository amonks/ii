package node

import (
	"context"
	"testing"

	pb "monks.co/apps/breadcrumbs/proto"
)

func TestSplitVisitsEmpty(t *testing.T) {
	segs := splitVisits(nil)
	if len(segs) != 0 {
		t.Errorf("got %d segments, want 0", len(segs))
	}
}

func TestSplitVisitsSinglePoint(t *testing.T) {
	segs := splitVisits([]*pb.Point{{Timestamp: 1000}})
	if len(segs) != 1 || len(segs[0]) != 1 {
		t.Errorf("got %d segments, want 1 with 1 point", len(segs))
	}
}

func TestSplitVisitsSingleSegment(t *testing.T) {
	pts := []*pb.Point{
		{Timestamp: 1_000_000_000},
		{Timestamp: 2_000_000_000},
		{Timestamp: 3_000_000_000},
	}
	segs := splitVisits(pts)
	if len(segs) != 1 {
		t.Errorf("got %d segments, want 1", len(segs))
	}
	if len(segs[0]) != 3 {
		t.Errorf("segment has %d points, want 3", len(segs[0]))
	}
}

func TestSplitVisitsTwoSegments(t *testing.T) {
	pts := []*pb.Point{
		{Timestamp: 1_000_000_000},
		{Timestamp: 2_000_000_000},
		// 5-minute gap
		{Timestamp: 302_000_000_000},
		{Timestamp: 303_000_000_000},
	}
	segs := splitVisits(pts)
	if len(segs) != 2 {
		t.Fatalf("got %d segments, want 2", len(segs))
	}
	if len(segs[0]) != 2 {
		t.Errorf("segment 0 has %d points, want 2", len(segs[0]))
	}
	if len(segs[1]) != 2 {
		t.Errorf("segment 1 has %d points, want 2", len(segs[1]))
	}
}

func TestSplitVisitsExactThreshold(t *testing.T) {
	pts := []*pb.Point{
		{Timestamp: 0},
		{Timestamp: visitGapNanos}, // exactly at threshold — should NOT split
	}
	segs := splitVisits(pts)
	if len(segs) != 1 {
		t.Errorf("got %d segments, want 1 (gap at exact threshold should not split)", len(segs))
	}
}

// mockNeighborFetcher implements neighborFetcher for testing.
type mockNeighborFetcher struct {
	prev map[int64]*pb.Point // beforeTimestamp → result
	next map[int64]*pb.Point // afterTimestamp → result
}

func (m *mockNeighborFetcher) PrevVisiblePoint(_ context.Context, beforeTimestamp int64, _ float64) (*pb.Point, error) {
	return m.prev[beforeTimestamp], nil
}

func (m *mockNeighborFetcher) NextVisiblePoint(_ context.Context, afterTimestamp int64, _ float64) (*pb.Point, error) {
	return m.next[afterTimestamp], nil
}

func TestExtendSegmentsMiddle(t *testing.T) {
	// A segment in the middle of the track should get both neighbors.
	segments := [][]*pb.Point{
		{{Timestamp: 5000, Latitude: 1}, {Timestamp: 6000, Latitude: 2}},
	}
	mock := &mockNeighborFetcher{
		prev: map[int64]*pb.Point{5000: {Timestamp: 4000, Latitude: 0}},
		next: map[int64]*pb.Point{6000: {Timestamp: 7000, Latitude: 3}},
	}
	result, err := extendSegments(context.Background(), segments, 0, 1000, 9000, mock)
	if err != nil {
		t.Fatal(err)
	}
	if len(result[0]) != 4 {
		t.Fatalf("segment has %d points, want 4", len(result[0]))
	}
	if result[0][0].Timestamp != 4000 {
		t.Errorf("first point timestamp = %d, want 4000", result[0][0].Timestamp)
	}
	if result[0][3].Timestamp != 7000 {
		t.Errorf("last point timestamp = %d, want 7000", result[0][3].Timestamp)
	}
}

func TestExtendSegmentsGlobalEndpoints(t *testing.T) {
	// A segment at the global start shouldn't get a prev neighbor.
	// A segment at the global end shouldn't get a next neighbor.
	segments := [][]*pb.Point{
		{{Timestamp: 1000}, {Timestamp: 2000}},
	}
	mock := &mockNeighborFetcher{
		prev: map[int64]*pb.Point{},
		next: map[int64]*pb.Point{},
	}
	result, err := extendSegments(context.Background(), segments, 0, 1000, 2000, mock)
	if err != nil {
		t.Fatal(err)
	}
	if len(result[0]) != 2 {
		t.Errorf("segment has %d points, want 2 (no extension at global endpoints)", len(result[0]))
	}
}

func TestExtendSegmentsMultiple(t *testing.T) {
	// Two segments, first at global start, second at global end.
	segments := [][]*pb.Point{
		{{Timestamp: 1000}, {Timestamp: 2000}},
		{{Timestamp: 5000}, {Timestamp: 9000}},
	}
	mock := &mockNeighborFetcher{
		next: map[int64]*pb.Point{2000: {Timestamp: 3000}},
		prev: map[int64]*pb.Point{5000: {Timestamp: 4000}},
	}
	result, err := extendSegments(context.Background(), segments, 0, 1000, 9000, mock)
	if err != nil {
		t.Fatal(err)
	}
	// First segment: no prev (global start), has next.
	if len(result[0]) != 3 {
		t.Errorf("segment 0 has %d points, want 3", len(result[0]))
	}
	// Second segment: has prev, no next (global end).
	if len(result[1]) != 3 {
		t.Errorf("segment 1 has %d points, want 3", len(result[1]))
	}
}
