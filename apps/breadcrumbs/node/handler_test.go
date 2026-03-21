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

	// Low zoom (high threshold) should filter out the low-sig point.
	req := httptest.NewRequest("GET", "/tiles/0/0/0", nil)
	req.Header.Set("Accept", "application/protobuf")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var track pb.Track
	proto.Unmarshal(w.Body.Bytes(), &track)
	if len(track.Points) != 1 {
		t.Errorf("low zoom: got %d points, want 1", len(track.Points))
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

func TestHandlerEventsMissingClient(t *testing.T) {
	h := testHandler(t)

	req := httptest.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
