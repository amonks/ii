package node

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	pb "monks.co/apps/breadcrumbs/proto"
	"google.golang.org/protobuf/proto"
)

func TestNewNode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	n, err := NewNode(context.Background(), Config{
		DBPath:   dbPath,
		Capacity: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	if n.Handler() == nil {
		t.Fatal("Handler() returned nil")
	}
}

func TestNewNodeBadPath(t *testing.T) {
	_, err := NewNode(context.Background(), Config{
		DBPath: "/nonexistent/dir/test.db",
	})
	if err == nil {
		t.Fatal("expected error for bad db path")
	}
}

func TestNodeIntegration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	n, err := NewNode(context.Background(), Config{
		DBPath:   dbPath,
		Capacity: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	h := n.Handler()

	// Use widely separated points so VW gives high significance.
	track := &pb.Track{
		Points: []*pb.Point{
			{Timestamp: 1000, Latitude: -40, Longitude: -80},
			{Timestamp: 2000, Latitude: 40, Longitude: 0},
			{Timestamp: 3000, Latitude: -40, Longitude: 80},
		},
	}
	body, _ := proto.Marshal(track)
	req := httptest.NewRequest("POST", "/ingest", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("ingest status = %d, body = %s", w.Code, w.Body.String())
	}

	// Query at zoom 0 — widely-spaced points have high significance.
	req = httptest.NewRequest("GET", "/tiles/0/0/0", nil)
	req.Header.Set("Accept", "application/protobuf")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("tile status = %d", w.Code)
	}

	var result pb.Track
	if err := proto.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Points) != 3 {
		t.Errorf("got %d points, want 3", len(result.Points))
	}
}

func TestNodeRecovery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Create node, ingest points, close.
	n1, err := NewNode(context.Background(), Config{
		DBPath:   dbPath,
		Capacity: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Use widely separated points for high VW significance.
	track := &pb.Track{
		Points: []*pb.Point{
			{Timestamp: 1000, Latitude: -40, Longitude: -80},
			{Timestamp: 2000, Latitude: 40, Longitude: 0},
			{Timestamp: 3000, Latitude: -40, Longitude: 80},
		},
	}
	body, _ := proto.Marshal(track)
	req := httptest.NewRequest("POST", "/ingest", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	n1.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("ingest status = %d", w.Code)
	}
	n1.Close()

	// Reopen node — simplifier should recover from last two points.
	n2, err := NewNode(context.Background(), Config{
		DBPath:   dbPath,
		Capacity: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n2.Close()

	// Ingest another widely-separated point.
	track2 := &pb.Track{
		Points: []*pb.Point{
			{Timestamp: 4000, Latitude: 40, Longitude: -80},
		},
	}
	body2, _ := proto.Marshal(track2)
	req = httptest.NewRequest("POST", "/ingest", strings.NewReader(string(body2)))
	w = httptest.NewRecorder()
	n2.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("ingest after recovery status = %d", w.Code)
	}

	// Verify all 4 points are queryable at zoom 0.
	req = httptest.NewRequest("GET", "/tiles/0/0/0", nil)
	req.Header.Set("Accept", "application/protobuf")
	w = httptest.NewRecorder()
	n2.Handler().ServeHTTP(w, req)

	var result pb.Track
	proto.Unmarshal(w.Body.Bytes(), &result)
	if len(result.Points) != 4 {
		t.Errorf("got %d points after recovery, want 4", len(result.Points))
	}
}
