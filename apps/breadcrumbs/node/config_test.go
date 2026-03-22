package node

import (
	"testing"
)

func TestParseConfig(t *testing.T) {
	data := []byte(`{
		"db_path": "/data/breadcrumbs.db",
		"listen": "127.0.0.1:8080",
		"upstream": "https://maps.example.com",
		"capacity": 100000,
		"subscriptions": [
			{"bbox": [-180, -90, 180, 90], "min_significance": 1e-4},
			{"bbox": [-74.0, 40.7, -73.9, 40.8], "min_significance": 0}
		]
	}`)

	c, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if c.DBPath != "/data/breadcrumbs.db" {
		t.Errorf("DBPath = %q, want %q", c.DBPath, "/data/breadcrumbs.db")
	}
	if c.Listen != "127.0.0.1:8080" {
		t.Errorf("Listen = %q, want %q", c.Listen, "127.0.0.1:8080")
	}
	if c.Upstream != "https://maps.example.com" {
		t.Errorf("Upstream = %q", c.Upstream)
	}
	if c.Capacity != 100000 {
		t.Errorf("Capacity = %d", c.Capacity)
	}
	if len(c.Subscriptions) != 2 {
		t.Fatalf("len(Subscriptions) = %d", len(c.Subscriptions))
	}
	if c.Subscriptions[0].BBox != [4]float64{-180, -90, 180, 90} {
		t.Errorf("sub[0].BBox = %v", c.Subscriptions[0].BBox)
	}
	if c.Subscriptions[0].MinSignificance != 1e-4 {
		t.Errorf("sub[0].MinSignificance = %v", c.Subscriptions[0].MinSignificance)
	}
	if c.Subscriptions[1].MinSignificance != 0 {
		t.Errorf("sub[1].MinSignificance = %v", c.Subscriptions[1].MinSignificance)
	}
}

func TestParseConfigNoUpstream(t *testing.T) {
	data := []byte(`{
		"db_path": "/data/breadcrumbs.db",
		"listen": ":8080",
		"capacity": 50000,
		"subscriptions": []
	}`)

	c, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if c.Upstream != "" {
		t.Errorf("Upstream = %q, want empty", c.Upstream)
	}
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.Upstream != "" {
		t.Errorf("Upstream = %q, want empty (root node)", c.Upstream)
	}
	if c.Capacity != 0 {
		t.Errorf("Capacity = %d, want 0 (no eviction)", c.Capacity)
	}
	if len(c.Subscriptions) != 1 {
		t.Fatalf("len(Subscriptions) = %d, want 1", len(c.Subscriptions))
	}
	if c.Subscriptions[0].BBox != [4]float64{-180, -90, 180, 90} {
		t.Errorf("BBox = %v, want global", c.Subscriptions[0].BBox)
	}
	if c.Subscriptions[0].MinSignificance != 0 {
		t.Errorf("MinSignificance = %v, want 0 (retain all)", c.Subscriptions[0].MinSignificance)
	}
}

func TestParseConfigInvalid(t *testing.T) {
	_, err := ParseConfig([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
