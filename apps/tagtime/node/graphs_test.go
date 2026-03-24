package node

import (
	"math"
	"testing"
	"time"
)

func TestComputeGraphDataBasic(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "#code"},
		{Timestamp: start.Add(2 * time.Hour).Unix(), Blurb: "#code #meeting"},
		{Timestamp: start.Add(25 * time.Hour).Unix(), Blurb: "#sleeping"},
	}

	data := ComputeGraphData(pings, changes, "day", start, end)

	if len(data.Buckets) != 2 {
		t.Fatalf("expected 2 day buckets, got %d", len(data.Buckets))
	}

	// Day 1: two pings with #code (2700*2=5400s), one ping with #meeting (2700s).
	day1 := data.Buckets[0]
	if math.Abs(day1.Tags["code"]-5400) > 0.1 {
		t.Errorf("day1 code = %.0f, want 5400", day1.Tags["code"])
	}
	if math.Abs(day1.Tags["meeting"]-2700) > 0.1 {
		t.Errorf("day1 meeting = %.0f, want 2700", day1.Tags["meeting"])
	}

	// Day 2: one ping with #sleeping.
	day2 := data.Buckets[1]
	if math.Abs(day2.Tags["sleeping"]-2700) > 0.1 {
		t.Errorf("day2 sleeping = %.0f, want 2700", day2.Tags["sleeping"])
	}
}

func TestComputeGraphDataUntagged(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 900}}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "just working no tags"},
	}

	data := ComputeGraphData(pings, changes, "day", start, end)
	if data.Buckets[0].Tags["untagged"] != 900 {
		t.Errorf("untagged = %.0f, want 900", data.Buckets[0].Tags["untagged"])
	}
}

func TestComputeGraphDataEmptyBlurb(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: ""},
	}

	data := ComputeGraphData(pings, changes, "day", start, end)
	if len(data.Buckets[0].Tags) != 0 {
		t.Errorf("empty blurb should not contribute to any tag, got %v", data.Buckets[0].Tags)
	}
}

func TestComputeGraphDataPeriodChange(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	mid := start.Add(12 * time.Hour)
	changes := []PeriodChange{
		{Timestamp: 0, Seed: 42, PeriodSecs: 2700},
		{Timestamp: mid.Unix(), Seed: 42, PeriodSecs: 900},
	}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "#code"},   // period=2700
		{Timestamp: start.Add(13 * time.Hour).Unix(), Blurb: "#code"},  // period=900
	}

	data := ComputeGraphData(pings, changes, "day", start, end)
	// First ping at period 2700, second at 900.
	codeTime := data.Buckets[0].Tags["code"]
	if math.Abs(codeTime-3600) > 0.1 {
		t.Errorf("code time = %.0f, want 3600 (2700+900)", codeTime)
	}
}

func TestEffectivePeriodAt(t *testing.T) {
	changes := []PeriodChange{
		{Timestamp: 1000, Seed: 42, PeriodSecs: 2700},
		{Timestamp: 5000, Seed: 42, PeriodSecs: 900},
	}

	tests := []struct {
		ts   int64
		want int64
	}{
		{500, 2700},  // before first change: default
		{1000, 2700}, // at first change
		{3000, 2700}, // between changes
		{5000, 900},  // at second change
		{9000, 900},  // after second change
	}

	for _, tt := range tests {
		got := effectivePeriodAt(changes, tt.ts)
		if got != tt.want {
			t.Errorf("effectivePeriodAt(%d) = %d, want %d", tt.ts, got, tt.want)
		}
	}
}
