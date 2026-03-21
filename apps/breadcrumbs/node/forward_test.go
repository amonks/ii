package node

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "monks.co/apps/breadcrumbs/proto"
	"google.golang.org/protobuf/proto"
)

func TestWatermark(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Default watermark is 0.
	wm, err := s.GetWatermark(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if wm != 0 {
		t.Errorf("default watermark = %d, want 0", wm)
	}

	// Set and get.
	if err := s.SetWatermark(ctx, 5000); err != nil {
		t.Fatal(err)
	}
	wm, err = s.GetWatermark(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if wm != 5000 {
		t.Errorf("watermark = %d, want 5000", wm)
	}

	// Update.
	if err := s.SetWatermark(ctx, 9000); err != nil {
		t.Fatal(err)
	}
	wm, _ = s.GetWatermark(ctx)
	if wm != 9000 {
		t.Errorf("watermark = %d, want 9000", wm)
	}
}

func TestForwardablePoints(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for i := range 5 {
		s.InsertPoint(ctx, &pb.Point{
			Timestamp: int64(i + 1),
			Latitude:  0, Longitude: 0,
		}, math.MaxFloat64, false)
	}

	// All points are forwardable from watermark 0.
	pts, err := s.ForwardablePoints(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 5 {
		t.Errorf("got %d points, want 5", len(pts))
	}

	// After watermark 3, only points 4 and 5 are forwardable.
	pts, err = s.ForwardablePoints(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 2 {
		t.Errorf("got %d points, want 2", len(pts))
	}
	if pts[0].Timestamp != 4 {
		t.Errorf("first forwardable = %d, want 4", pts[0].Timestamp)
	}
}

func TestForwarderForward(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Insert points to forward.
	for i := range 3 {
		s.InsertPoint(ctx, &pb.Point{
			Timestamp: int64(i + 1),
			Latitude:  0, Longitude: 0,
		}, math.MaxFloat64, false)
	}

	// Set up a fake upstream server.
	var receivedTrack pb.Track
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ingest" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", 404)
			return
		}
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		proto.Unmarshal(body, &receivedTrack)

		resp := &pb.IngestResponse{Watermark: 3}
		data, _ := proto.Marshal(resp)
		w.Write(data)
	}))
	defer upstream.Close()

	f := newForwarder(s, upstream.URL, 10000)
	n, wm, err := f.Forward(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("forwarded = %d, want 3", n)
	}
	if wm != 3 {
		t.Errorf("watermark = %d, want 3", wm)
	}
	if len(receivedTrack.Points) != 3 {
		t.Errorf("upstream received %d points, want 3", len(receivedTrack.Points))
	}

	// Watermark should be persisted.
	stored, _ := s.GetWatermark(ctx)
	if stored != 3 {
		t.Errorf("stored watermark = %d, want 3", stored)
	}

	// Second forward with no new points.
	n, _, err = f.Forward(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("second forward = %d, want 0", n)
	}
}

func TestForwarderUpstreamFailure(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.InsertPoint(ctx, &pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0}, math.MaxFloat64, false)

	// Upstream returns 500.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", 500)
	}))
	defer upstream.Close()

	f := newForwarder(s, upstream.URL, 10000)
	_, _, err := f.Forward(ctx)
	if err == nil {
		t.Fatal("expected error from upstream failure")
	}

	// Watermark should not have advanced.
	wm, _ := s.GetWatermark(ctx)
	if wm != 0 {
		t.Errorf("watermark = %d, should be 0 after failure", wm)
	}
}

func TestFetchTileFromUpstream(t *testing.T) {
	points := []*pb.Point{
		{Timestamp: 1, Latitude: 10, Longitude: 20},
		{Timestamp: 2, Latitude: 11, Longitude: 21},
	}
	track := &pb.Track{Points: points}
	body, _ := proto.Marshal(track)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tiles/5/10/15" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/protobuf" {
			t.Errorf("unexpected Accept: %s", r.Header.Get("Accept"))
		}
		w.Write(body)
	}))
	defer upstream.Close()

	got, err := FetchTileFromUpstream(context.Background(), http.DefaultClient, upstream.URL, 5, 10, 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d points, want 2", len(got))
	}
	if got[0].Latitude != 10 {
		t.Errorf("lat = %f, want 10", got[0].Latitude)
	}
}

func TestWriteUpstreamPoints(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	points := []*pb.Point{
		{Timestamp: 1, Latitude: 0, Longitude: 0},
		{Timestamp: 2, Latitude: 1, Longitude: 1},
	}

	changed, err := WriteUpstreamPoints(ctx, s, points, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected changed=true")
	}

	// Verify points are stored.
	pts, err := s.QueryTile(ctx, -90, 90, -180, 180, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 2 {
		t.Errorf("got %d points, want 2", len(pts))
	}
}
