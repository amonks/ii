package mobile

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	pb "monks.co/apps/breadcrumbs/proto"
	"google.golang.org/protobuf/proto"
)

func TestStartStop(t *testing.T) {
	dir := t.TempDir()
	config := fmt.Sprintf(`{"db_path": "%s/test.db", "capacity": 1000}`, dir)

	port, err := Start([]byte(config))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if port <= 0 {
		t.Fatalf("expected positive port, got %d", port)
	}

	// Ingest a point.
	track := &pb.Track{
		Points: []*pb.Point{{
			Timestamp: 1000000000,
			Latitude:  41.8781,
			Longitude: -87.6298,
		}},
	}
	body, err := proto.Marshal(track)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp, err := http.Post(
		fmt.Sprintf("http://127.0.0.1:%d/ingest", port),
		"application/protobuf",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("POST /ingest: %v", err)
	}

	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /ingest status %d: %s", resp.StatusCode, respBody)
	}

	var ingestResp pb.IngestResponse
	if err := proto.Unmarshal(respBody, &ingestResp); err != nil {
		t.Fatalf("unmarshal IngestResponse: %v", err)
	}
	if ingestResp.Watermark != 1000000000 {
		t.Fatalf("expected watermark 1000000000, got %d", ingestResp.Watermark)
	}

	// Stats should show 1 point.
	statsResp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/stats", port))
	if err != nil {
		t.Fatalf("GET /stats: %v", err)
	}
	statsBody, err := io.ReadAll(statsResp.Body)
	statsResp.Body.Close()
	if err != nil {
		t.Fatalf("reading stats: %v", err)
	}
	if statsResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /stats status %d: %s", statsResp.StatusCode, statsBody)
	}
	var stats pb.StatsResponse
	if err := proto.Unmarshal(statsBody, &stats); err != nil {
		t.Fatalf("unmarshal StatsResponse: %v", err)
	}
	if stats.Count != 1 {
		t.Fatalf("expected count 1, got %d", stats.Count)
	}
	if stats.LatestPoint == nil {
		t.Fatal("expected latest_point, got nil")
	}
	if stats.LatestPoint.Timestamp != 1000000000 {
		t.Fatalf("expected latest timestamp 1000000000, got %d", stats.LatestPoint.Timestamp)
	}
	if stats.LatestPoint.Latitude != 41.8781 {
		t.Fatalf("expected latest lat 41.8781, got %f", stats.LatestPoint.Latitude)
	}

	// Calling Start again should return the same port.
	port2, err := Start([]byte(config))
	if err != nil {
		t.Fatalf("Start again: %v", err)
	}
	if port2 != port {
		t.Fatalf("expected same port %d, got %d", port, port2)
	}

	Stop()

	// After stop, the port should be closed.
	_, err = http.Post(
		fmt.Sprintf("http://127.0.0.1:%d/ingest", port),
		"application/protobuf",
		bytes.NewReader(body),
	)
	if err == nil {
		t.Fatal("expected error after Stop, got nil")
	}
}
