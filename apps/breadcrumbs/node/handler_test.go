package node

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pb "monks.co/apps/breadcrumbs/proto"
	"google.golang.org/protobuf/proto"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	s := testStore(t)
	simp := NewSimplifier()
	prev, tail, _ := s.LastTwoPoints(context.Background())
	if tail != nil {
		simp.Recover(prev, tail)
	}
	hub := NewHub()
	config := &Config{Capacity: 10000}
	return newHandler(s, simp, hub, config, nil)
}

func TestHandlerIngest(t *testing.T) {
	h := testHandler(t)

	track := &pb.Track{
		Points: []*pb.Point{
			{Timestamp: 1000, Latitude: 41.88, Longitude: -87.63},
			{Timestamp: 2000, Latitude: 41.89, Longitude: -87.62},
			{Timestamp: 3000, Latitude: 41.90, Longitude: -87.61},
		},
	}
	body, _ := proto.Marshal(track)

	req := httptest.NewRequest("POST", "/ingest", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp pb.IngestResponse
	if err := proto.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Watermark != 3000 {
		t.Errorf("watermark = %d, want 3000", resp.Watermark)
	}
}

func TestHandlerIngestInvalidProtobuf(t *testing.T) {
	h := testHandler(t)
	req := httptest.NewRequest("POST", "/ingest", strings.NewReader("not protobuf"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandlerTileProtobuf(t *testing.T) {
	h := testHandler(t)

	// Ingest points.
	track := &pb.Track{
		Points: []*pb.Point{
			{Timestamp: 1000, Latitude: 41.88, Longitude: -87.63},
			{Timestamp: 2000, Latitude: 41.89, Longitude: -87.62},
		},
	}
	body, _ := proto.Marshal(track)
	req := httptest.NewRequest("POST", "/ingest", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("ingest status = %d", w.Code)
	}

	// Request tile as protobuf.
	req = httptest.NewRequest("GET", "/tiles/0/0/0", nil)
	req.Header.Set("Accept", "application/protobuf")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("tile status = %d, body = %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "application/protobuf" {
		t.Errorf("content-type = %q", w.Header().Get("Content-Type"))
	}

	var result pb.Track
	if err := proto.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Points) != 2 {
		t.Errorf("got %d points, want 2", len(result.Points))
	}
}

func TestHandlerTileMVT(t *testing.T) {
	h := testHandler(t)

	// Ingest a point.
	track := &pb.Track{
		Points: []*pb.Point{
			{Timestamp: 1000, Latitude: 0, Longitude: 0},
		},
	}
	body, _ := proto.Marshal(track)
	req := httptest.NewRequest("POST", "/ingest", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Request as MVT.
	req = httptest.NewRequest("GET", "/tiles/0/0/0", nil)
	req.Header.Set("Accept", "application/vnd.mapbox-vector-tile")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/vnd.mapbox-vector-tile" {
		t.Errorf("content-type = %q", w.Header().Get("Content-Type"))
	}

	// Verify it's valid MVT.
	var tile pb.Tile
	if err := proto.Unmarshal(w.Body.Bytes(), &tile); err != nil {
		t.Fatal(err)
	}
	if len(tile.Layers) != 1 || tile.Layers[0].GetName() != "track" {
		t.Error("invalid MVT layer")
	}
}

func TestHandlerTileDefaultMVT(t *testing.T) {
	h := testHandler(t)

	// Ingest a point.
	track := &pb.Track{
		Points: []*pb.Point{
			{Timestamp: 1000, Latitude: 0, Longitude: 0},
		},
	}
	body, _ := proto.Marshal(track)
	req := httptest.NewRequest("POST", "/ingest", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Request tile without Accept header — should default to MVT.
	req = httptest.NewRequest("GET", "/tiles/0/0/0", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/vnd.mapbox-vector-tile" {
		t.Errorf("content-type = %q, want application/vnd.mapbox-vector-tile", w.Header().Get("Content-Type"))
	}

	var tile pb.Tile
	if err := proto.Unmarshal(w.Body.Bytes(), &tile); err != nil {
		t.Fatal(err)
	}
	if len(tile.Layers) != 1 || tile.Layers[0].GetName() != "track" {
		t.Error("invalid MVT layer")
	}
}

func TestHandlerTileEmptyNoError(t *testing.T) {
	h := testHandler(t)

	req := httptest.NewRequest("GET", "/tiles/0/0/0", nil)
	req.Header.Set("Accept", "application/protobuf")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandlerTileInvalidCoords(t *testing.T) {
	h := testHandler(t)

	req := httptest.NewRequest("GET", "/tiles/0/1/0", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandlerTileSignificanceFiltering(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Insert a point with low significance directly.
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0}, 0.0001, false)
	// Insert a point with high significance.
	s.InsertPoint(ctx, &pb.Point{Timestamp: 2, Latitude: 0.01, Longitude: 0.01}, math.MaxFloat64, false)

	simp := NewSimplifier()
	hub := NewHub()
	config := &Config{Capacity: 10000}
	h := newHandler(s, simp, hub, config, nil)

	// Default (detail=5): threshold at z=0 = (360/256)^2 * 1e-5 ≈ 1.98e-5.
	// The low-sig point (0.0001) is above this, so both survive.
	req := httptest.NewRequest("GET", "/tiles/0/0/0", nil)
	req.Header.Set("Accept", "application/protobuf")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var track pb.Track
	proto.Unmarshal(w.Body.Bytes(), &track)
	if len(track.Points) != 2 {
		t.Errorf("default detail: got %d points, want 2", len(track.Points))
	}

	// detail=0: threshold at z=0 = (360/256)^2 ≈ 1.977, only high-sig survives.
	req = httptest.NewRequest("GET", "/tiles/0/0/0?detail=0", nil)
	req.Header.Set("Accept", "application/protobuf")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var filtered pb.Track
	proto.Unmarshal(w.Body.Bytes(), &filtered)
	if len(filtered.Points) != 1 {
		t.Errorf("detail=0: got %d points, want 1", len(filtered.Points))
	}
}

func TestHandlerTileBoundaryBuffer(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Insert points that cross the boundary between tile (1,0,0) and (1,1,0).
	// At z=1, tile (1,0,0) covers lon [-180, 0], tile (1,1,0) covers lon [0, 180].
	// Place points on either side of lon=0.
	s.InsertPoint(ctx, &pb.Point{Timestamp: 1, Latitude: 45, Longitude: -1}, math.MaxFloat64, false)
	s.InsertPoint(ctx, &pb.Point{Timestamp: 2, Latitude: 45, Longitude: 1}, math.MaxFloat64, false)

	simp := NewSimplifier()
	hub := NewHub()
	config := &Config{Capacity: 10000}
	h := newHandler(s, simp, hub, config, nil)

	// Tile (1,0,0) should include both points thanks to the buffer,
	// even though lon=1 is outside its strict bbox.
	req := httptest.NewRequest("GET", "/tiles/1/0/0", nil)
	req.Header.Set("Accept", "application/protobuf")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var track pb.Track
	proto.Unmarshal(w.Body.Bytes(), &track)
	if len(track.Points) != 2 {
		t.Errorf("tile (1,0,0): got %d points, want 2 (buffer should include cross-boundary point)", len(track.Points))
	}

	// Tile (1,1,0) should also include both points.
	req = httptest.NewRequest("GET", "/tiles/1/1/0", nil)
	req.Header.Set("Accept", "application/protobuf")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var track2 pb.Track
	proto.Unmarshal(w.Body.Bytes(), &track2)
	if len(track2.Points) != 2 {
		t.Errorf("tile (1,1,0): got %d points, want 2 (buffer should include cross-boundary point)", len(track2.Points))
	}
}

func TestHandlerFlush(t *testing.T) {
	h := testHandler(t)

	req := httptest.NewRequest("POST", "/flush", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	var resp pb.FlushResponse
	if err := proto.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Watermark != 0 {
		t.Errorf("watermark = %d", resp.Watermark)
	}
}

func TestHandlerEvents(t *testing.T) {
	s := testStore(t)
	simp := NewSimplifier()
	hub := NewHub()
	config := &Config{}
	h := newHandler(s, simp, hub, config, nil).(*handler)

	server := httptest.NewServer(h)
	defer server.Close()

	// Connect SSE client.
	resp, err := http.Get(server.URL + "/events?client=test123")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("content-type = %q", resp.Header.Get("Content-Type"))
	}

	// Give SSE connection time to establish.
	time.Sleep(10 * time.Millisecond)

	// Publish an event.
	h.notifyTileUpdated("test123", 3, 2, 1)

	// Read the event.
	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	got := string(buf[:n])
	if !strings.Contains(got, "event: tile-updated") {
		t.Errorf("expected tile-updated event, got: %q", got)
	}
	if !strings.Contains(got, `"z":3`) {
		t.Errorf("expected z:3 in event data, got: %q", got)
	}
}

func TestHandlerStatsEmpty(t *testing.T) {
	h := testHandler(t)

	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp pb.StatsResponse
	if err := proto.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 0 {
		t.Errorf("count = %d, want 0", resp.Count)
	}
	if resp.LatestPoint != nil {
		t.Errorf("latest_point should be nil, got %v", resp.LatestPoint)
	}
}

func TestHandlerStatsAfterIngest(t *testing.T) {
	h := testHandler(t)

	// Ingest some points.
	track := &pb.Track{
		Points: []*pb.Point{
			{Timestamp: 1000, Latitude: 41.88, Longitude: -87.63},
			{Timestamp: 3000, Latitude: 41.90, Longitude: -87.61},
			{Timestamp: 2000, Latitude: 41.89, Longitude: -87.62},
		},
	}
	body, _ := proto.Marshal(track)
	req := httptest.NewRequest("POST", "/ingest", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("ingest status = %d", w.Code)
	}

	// Now check stats.
	req = httptest.NewRequest("GET", "/stats", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp pb.StatsResponse
	if err := proto.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Errorf("count = %d, want 3", resp.Count)
	}
	if resp.LatestPoint == nil {
		t.Fatal("latest_point is nil")
	}
	if resp.LatestPoint.Timestamp != 3000 {
		t.Errorf("latest timestamp = %d, want 3000", resp.LatestPoint.Timestamp)
	}
	if resp.LatestPoint.Latitude != 41.90 {
		t.Errorf("latest lat = %f, want 41.90", resp.LatestPoint.Latitude)
	}
}

func TestHandlerStatsForwardQueue(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Insert 5 points.
	for i := range 5 {
		s.InsertPoint(ctx, &pb.Point{
			Timestamp: int64(i + 1),
			Latitude:  0, Longitude: 0,
		}, math.MaxFloat64, false)
	}

	// Set watermark to 3 — points 4 and 5 are unforwarded.
	s.SetWatermark(ctx, 3)

	simp := NewSimplifier()
	hub := NewHub()
	config := &Config{Capacity: 10000}
	h := newHandler(s, simp, hub, config, nil)

	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp pb.StatsResponse
	if err := proto.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 5 {
		t.Errorf("count = %d, want 5", resp.Count)
	}
	if resp.ForwardWatermark != 3 {
		t.Errorf("forward_watermark = %d, want 3", resp.ForwardWatermark)
	}
	if resp.ForwardQueueSize != 2 {
		t.Errorf("forward_queue_size = %d, want 2", resp.ForwardQueueSize)
	}
}

func TestHandlerEventsMissingClient(t *testing.T) {
	h := testHandler(t)

	req := httptest.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandlerRecompute(t *testing.T) {
	h := testHandler(t)

	// Ingest collinear points — pure VW gives them zero significance.
	track := &pb.Track{
		Points: []*pb.Point{
			{Timestamp: 1, Latitude: 0, Longitude: 0},
			{Timestamp: 2, Latitude: 1, Longitude: 1},
			{Timestamp: 3, Latitude: 2, Longitude: 2},
		},
	}
	body, _ := proto.Marshal(track)
	req := httptest.NewRequest("POST", "/ingest", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("ingest status = %d", w.Code)
	}

	// The middle point (1,1) has zero significance from pure area VW.
	// At z=0 default detail=5, threshold ≈ 1.977e-5. Middle point (sig=0) filtered.
	req = httptest.NewRequest("GET", "/tiles/0/0/0", nil)
	req.Header.Set("Accept", "application/protobuf")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var before pb.Track
	proto.Unmarshal(w.Body.Bytes(), &before)
	if len(before.Points) != 2 {
		t.Errorf("before recompute: got %d points, want 2 (middle filtered)", len(before.Points))
	}

	// Recompute with distance_floor.
	req = httptest.NewRequest("POST", "/recompute?method=distance_floor", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("recompute status = %d, body = %s", w.Code, w.Body.String())
	}

	// Now the middle point has sig = distanceSquared((0,0),(2,2)) = 8,
	// well above threshold. All 3 points should survive.
	req = httptest.NewRequest("GET", "/tiles/0/0/0", nil)
	req.Header.Set("Accept", "application/protobuf")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var after pb.Track
	proto.Unmarshal(w.Body.Bytes(), &after)
	if len(after.Points) != 3 {
		t.Errorf("after recompute: got %d points, want 3 (middle should survive)", len(after.Points))
	}
}

func TestHandlerRecomputeInvalidMethod(t *testing.T) {
	h := testHandler(t)
	req := httptest.NewRequest("POST", "/recompute?method=bogus", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
