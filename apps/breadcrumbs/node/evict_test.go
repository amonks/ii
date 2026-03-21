package node

import (
	"context"
	"math"
	"testing"

	pb "monks.co/apps/breadcrumbs/proto"
)

func TestEvictOverCapacity(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Insert 10 points.
	for i := range 10 {
		s.InsertPoint(ctx, &pb.Point{
			Timestamp: int64(i + 1),
			Latitude:  0, Longitude: 0,
		}, math.MaxFloat64, false)
	}

	// Evict with capacity 7, watermark covers all.
	evicted, err := s.Evict(ctx, 7, math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	if evicted != 3 {
		t.Errorf("evicted = %d, want 3", evicted)
	}

	// Should have 7 points remaining.
	pts, err := s.QueryTile(ctx, -90, 90, -180, 180, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 7 {
		t.Errorf("remaining = %d, want 7", len(pts))
	}
}

func TestEvictProtectsSubscribed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Insert 5 subscribed + 5 unsubscribed.
	for i := range 5 {
		s.InsertPoint(ctx, &pb.Point{
			Timestamp: int64(i + 1),
			Latitude:  0, Longitude: 0,
		}, math.MaxFloat64, true) // subscribed
	}
	for i := range 5 {
		s.InsertPoint(ctx, &pb.Point{
			Timestamp: int64(i + 6),
			Latitude:  0, Longitude: 0,
		}, math.MaxFloat64, false) // unsubscribed
	}

	// Evict with capacity 2 for unsubscribed.
	evicted, err := s.Evict(ctx, 2, math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	if evicted != 3 {
		t.Errorf("evicted = %d, want 3", evicted)
	}

	// Should have 5 subscribed + 2 unsubscribed = 7 remaining.
	pts, err := s.QueryTile(ctx, -90, 90, -180, 180, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 7 {
		t.Errorf("remaining = %d, want 7", len(pts))
	}
}

func TestEvictProtectsWatermark(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for i := range 10 {
		s.InsertPoint(ctx, &pb.Point{
			Timestamp: int64(i + 1),
			Latitude:  0, Longitude: 0,
		}, math.MaxFloat64, false)
	}

	// Watermark at 5: only points 1-5 are eligible for eviction.
	evicted, err := s.Evict(ctx, 2, 5)
	if err != nil {
		t.Fatal(err)
	}
	if evicted != 3 {
		t.Errorf("evicted = %d, want 3", evicted)
	}

	// All 5 points past watermark should remain + 2 within capacity.
	pts, err := s.QueryTile(ctx, -90, 90, -180, 180, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 7 {
		t.Errorf("remaining = %d, want 7", len(pts))
	}
}

func TestRecomputeSubscriptions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Chicago
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1, Latitude: 41.88, Longitude: -87.63}, 1.0, false)
	// New York
	s.InsertPoint(ctx, &pb.Point{Timestamp: 2, Latitude: 40.71, Longitude: -74.01}, 1.0, false)
	// Low significance point in Chicago
	s.InsertPoint(ctx, &pb.Point{Timestamp: 3, Latitude: 41.89, Longitude: -87.62}, 0.001, false)

	// Subscribe to Chicago area with min_significance 0.5.
	err := s.RecomputeSubscriptions(ctx, []Subscription{
		{BBox: [4]float64{-88.0, 41.0, -87.0, 42.0}, MinSignificance: 0.5},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check that only high-sig Chicago point is subscribed.
	var sub1, sub2, sub3 int
	s.db.QueryRow("SELECT subscribed FROM points WHERE timestamp = 1").Scan(&sub1)
	s.db.QueryRow("SELECT subscribed FROM points WHERE timestamp = 2").Scan(&sub2)
	s.db.QueryRow("SELECT subscribed FROM points WHERE timestamp = 3").Scan(&sub3)

	if sub1 != 1 {
		t.Errorf("Chicago high-sig: subscribed = %d, want 1", sub1)
	}
	if sub2 != 0 {
		t.Errorf("New York: subscribed = %d, want 0", sub2)
	}
	if sub3 != 0 {
		t.Errorf("Chicago low-sig: subscribed = %d, want 0", sub3)
	}
}

func TestRecomputeSubscriptionsUpdate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.InsertPoint(ctx, &pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0}, 1.0, true)

	// Remove all subscriptions.
	if err := s.RecomputeSubscriptions(ctx, nil); err != nil {
		t.Fatal(err)
	}

	var sub int
	s.db.QueryRow("SELECT subscribed FROM points WHERE timestamp = 1").Scan(&sub)
	if sub != 0 {
		t.Errorf("after clearing subs: subscribed = %d, want 0", sub)
	}
}
