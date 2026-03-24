package node

import (
	"context"
	"io"
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

func TestForwarderBatching(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Insert more points than one batch (forwardBatchSize = 1000).
	n := forwardBatchSize + 500
	for i := range n {
		s.InsertPoint(ctx, &pb.Point{
			Timestamp: int64(i + 1),
			Latitude:  0, Longitude: 0,
		}, math.MaxFloat64, false)
	}

	// Track how many requests the upstream receives.
	var batchCount int
	var totalReceived int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var track pb.Track
		proto.Unmarshal(body, &track)
		batchCount++
		totalReceived += len(track.Points)

		resp := &pb.IngestResponse{Watermark: track.Points[len(track.Points)-1].Timestamp}
		data, _ := proto.Marshal(resp)
		w.Write(data)
	}))
	defer upstream.Close()

	f := newForwarder(s, upstream.URL, 100000)
	forwarded, wm, err := f.Forward(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if int(forwarded) != n {
		t.Errorf("forwarded = %d, want %d", forwarded, n)
	}
	if wm != int64(n) {
		t.Errorf("watermark = %d, want %d", wm, n)
	}
	if batchCount != 2 {
		t.Errorf("batch count = %d, want 2", batchCount)
	}
	if totalReceived != n {
		t.Errorf("total received = %d, want %d", totalReceived, n)
	}

	// Watermark should be persisted at the final value.
	stored, _ := s.GetWatermark(ctx)
	if stored != int64(n) {
		t.Errorf("stored watermark = %d, want %d", stored, n)
	}
}

func TestForwarderBatchPartialFailure(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Insert 2.5 batches of points.
	n := forwardBatchSize*2 + 500
	for i := range n {
		s.InsertPoint(ctx, &pb.Point{
			Timestamp: int64(i + 1),
			Latitude:  0, Longitude: 0,
		}, math.MaxFloat64, false)
	}

	// Upstream fails on the second batch.
	var batchCount int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batchCount++
		if batchCount == 2 {
			http.Error(w, "server error", 500)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var track pb.Track
		proto.Unmarshal(body, &track)

		resp := &pb.IngestResponse{Watermark: track.Points[len(track.Points)-1].Timestamp}
		data, _ := proto.Marshal(resp)
		w.Write(data)
	}))
	defer upstream.Close()

	f := newForwarder(s, upstream.URL, 100000)
	forwarded, wm, err := f.Forward(ctx)
	if err == nil {
		t.Fatal("expected error from partial failure")
	}

	// First batch should have been forwarded.
	if int(forwarded) != forwardBatchSize {
		t.Errorf("forwarded = %d, want %d", forwarded, forwardBatchSize)
	}
	// Watermark should be at end of first batch.
	if wm != int64(forwardBatchSize) {
		t.Errorf("watermark = %d, want %d", wm, forwardBatchSize)
	}

	// Stored watermark should also reflect partial progress.
	stored, _ := s.GetWatermark(ctx)
	if stored != int64(forwardBatchSize) {
		t.Errorf("stored watermark = %d, want %d", stored, forwardBatchSize)
	}
}

func TestForwarderRealisticTimestamps(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Use realistic nanosecond timestamps like iOS produces.
	baseTS := int64(1774200000000000000) // ~March 2026 in nanoseconds
	for i := range 5 {
		s.InsertPoint(ctx, &pb.Point{
			Timestamp: baseTS + int64(i)*1_000_000_000,
			Latitude:  41.88 + float64(i)*0.001,
			Longitude: -87.63,
		}, math.MaxFloat64, false)
	}

	// Queue should contain all 5 points.
	queueSize, err := s.ForwardQueueSize(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if queueSize != 5 {
		t.Errorf("queue size = %d, want 5", queueSize)
	}

	// Forward to upstream.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		resp := &pb.IngestResponse{}
		data, _ := proto.Marshal(resp)
		w.Write(data)
	}))
	defer upstream.Close()

	f := newForwarder(s, upstream.URL, 100000)
	forwarded, wm, err := f.Forward(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if forwarded != 5 {
		t.Errorf("forwarded = %d, want 5", forwarded)
	}
	expectedWM := baseTS + 4*1_000_000_000
	if wm != expectedWM {
		t.Errorf("watermark = %d, want %d", wm, expectedWM)
	}

	// Queue should now be empty.
	queueSize, err = s.ForwardQueueSize(ctx, wm)
	if err != nil {
		t.Fatal(err)
	}
	if queueSize != 0 {
		t.Errorf("queue size after forward = %d, want 0", queueSize)
	}

	// Watermark should be readable back correctly.
	storedWM, err := s.GetWatermark(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if storedWM != expectedWM {
		t.Errorf("stored watermark = %d, want %d", storedWM, expectedWM)
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
