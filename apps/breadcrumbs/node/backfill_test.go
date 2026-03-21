package node

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	pb "monks.co/apps/breadcrumbs/proto"
	"google.golang.org/protobuf/proto"
)

func TestZoomForSignificance(t *testing.T) {
	// min_significance 0 → max zoom
	if z := zoomForSignificance(0); z != 22 {
		t.Errorf("sig=0: z=%d, want 22", z)
	}

	// The threshold at zoom 0 (detail=0) is 360/256 ≈ 1.406.
	// So significance >= 1.406 should yield zoom 0.
	if z := zoomForSignificance(2.0); z != 0 {
		t.Errorf("sig=2.0: z=%d, want 0", z)
	}

	// Check round-trip: the threshold at the returned zoom should be <= minSig.
	for _, minSig := range []float64{1e-4, 0.001, 0.01, 0.1, 1.0} {
		z := zoomForSignificance(minSig)
		threshold := SignificanceThreshold(z, 0)
		if threshold > minSig*1.01 { // small tolerance
			t.Errorf("sig=%g: z=%d, threshold=%g > minSig", minSig, z, threshold)
		}
	}
}

func TestLonToTileX(t *testing.T) {
	// At zoom 1, -180 → 0, 0 → 1, 180 → 1 (clamped)
	if x := lonToTileX(-180, 1); x != 0 {
		t.Errorf("lonToTileX(-180, 1) = %d, want 0", x)
	}
	if x := lonToTileX(0, 1); x != 1 {
		t.Errorf("lonToTileX(0, 1) = %d, want 1", x)
	}
}

func TestLatToTileY(t *testing.T) {
	// At zoom 1, equator → y=1
	y := latToTileY(0, 1)
	if y != 1 {
		t.Errorf("latToTileY(0, 1) = %d, want 1", y)
	}
}

func TestBackfillSubscriptions(t *testing.T) {
	s := testStore(t)

	var fetchCount atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		// Return a point for every tile request.
		track := &pb.Track{
			Points: []*pb.Point{
				{Timestamp: int64(fetchCount.Load()), Latitude: 0, Longitude: 0},
			},
		}
		data, _ := proto.Marshal(track)
		w.Write(data)
	}))
	defer upstream.Close()

	config := Config{
		Upstream: upstream.URL,
		Capacity: 10000,
		Subscriptions: []Subscription{
			// Small bbox, high min_significance → low zoom → few tiles.
			{BBox: [4]float64{-1, -1, 1, 1}, MinSignificance: 1.0},
		},
	}

	backfillSubscriptions(context.Background(), s, config)

	// Should have fetched at least 1 tile.
	if fetchCount.Load() == 0 {
		t.Error("expected at least one tile fetch")
	}

	// Should have stored points.
	pts, err := s.QueryTile(context.Background(), -90, 90, -180, 180, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) == 0 {
		t.Error("expected backfilled points in store")
	}
}

func TestBackfillCancellation(t *testing.T) {
	s := testStore(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		track := &pb.Track{}
		data, _ := proto.Marshal(track)
		w.Write(data)
	}))
	defer upstream.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	config := Config{
		Upstream: upstream.URL,
		Subscriptions: []Subscription{
			{BBox: [4]float64{-180, -90, 180, 90}, MinSignificance: 0},
		},
	}

	// Should return without blocking.
	backfillSubscriptions(ctx, s, config)

	// No points should be stored (cancelled before any fetch).
	pts, _ := s.QueryTile(context.Background(), -90, 90, -180, 180, 0)
	if len(pts) != 0 {
		t.Errorf("expected 0 points after cancelled backfill, got %d", len(pts))
	}
}

func TestZoomForSignificanceBounds(t *testing.T) {
	// Very large significance → low zoom.
	if z := zoomForSignificance(math.MaxFloat64); z != 0 {
		t.Errorf("sig=MaxFloat64: z=%d, want 0", z)
	}

	// Very small significance → high zoom (capped at 22).
	if z := zoomForSignificance(1e-20); z != 22 {
		t.Errorf("sig=1e-20: z=%d, want 22", z)
	}
}
