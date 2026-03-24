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

	data := ComputeGraphData(pings, changes, "day", start, end, nil)

	if len(data.Buckets) != 2 {
		t.Fatalf("expected 2 day buckets, got %d", len(data.Buckets))
	}

	// Day 1: two pings with #code, one with #meeting.
	// #code appears in both pings (2 hits), #meeting in one (1 hit).
	// Total tag-hits = 3, so code = 2/3*100 ≈ 66.67%, meeting = 1/3*100 ≈ 33.33%.
	day1 := data.Buckets[0]
	if math.Abs(day1.Tags["code"]-66.666666) > 0.1 {
		t.Errorf("day1 code = %.2f%%, want ~66.67%%", day1.Tags["code"])
	}
	if math.Abs(day1.Tags["meeting"]-33.333333) > 0.1 {
		t.Errorf("day1 meeting = %.2f%%, want ~33.33%%", day1.Tags["meeting"])
	}

	// Day 2: one ping with #sleeping = 100%.
	day2 := data.Buckets[1]
	if math.Abs(day2.Tags["sleeping"]-100) > 0.1 {
		t.Errorf("day2 sleeping = %.2f%%, want 100%%", day2.Tags["sleeping"])
	}
}

func TestComputeGraphDataUntagged(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 900}}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "just working no tags"},
	}

	data := ComputeGraphData(pings, changes, "day", start, end, nil)
	if math.Abs(data.Buckets[0].Tags["untagged"]-100) > 0.1 {
		t.Errorf("untagged = %.2f%%, want 100%%", data.Buckets[0].Tags["untagged"])
	}
}

func TestComputeGraphDataEmptyBlurb(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: ""},
	}

	data := ComputeGraphData(pings, changes, "day", start, end, nil)
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
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "#code"},  // period=2700
		{Timestamp: start.Add(13 * time.Hour).Unix(), Blurb: "#code"}, // period=900
	}

	data := ComputeGraphData(pings, changes, "day", start, end, nil)
	// Both pings are #code, so code = 100% regardless of period change.
	codePercent := data.Buckets[0].Tags["code"]
	if math.Abs(codePercent-100) > 0.1 {
		t.Errorf("code = %.2f%%, want 100%%", codePercent)
	}
}

func TestComputeGraphDataPeriodChangeAffectsWeight(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	mid := start.Add(12 * time.Hour)
	changes := []PeriodChange{
		{Timestamp: 0, Seed: 42, PeriodSecs: 2700},
		{Timestamp: mid.Unix(), Seed: 42, PeriodSecs: 900},
	}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "#code"},    // period=2700
		{Timestamp: start.Add(13 * time.Hour).Unix(), Blurb: "#sleep"},  // period=900
	}

	data := ComputeGraphData(pings, changes, "day", start, end, nil)
	// Weighted: code=2700, sleep=900, total=3600.
	// code=75%, sleep=25%.
	codePercent := data.Buckets[0].Tags["code"]
	sleepPercent := data.Buckets[0].Tags["sleep"]
	if math.Abs(codePercent-75) > 0.1 {
		t.Errorf("code = %.2f%%, want 75%%", codePercent)
	}
	if math.Abs(sleepPercent-25) > 0.1 {
		t.Errorf("sleep = %.2f%%, want 25%%", sleepPercent)
	}
}

func TestComputeGraphDataTagColors(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "#code #meeting"},
	}

	data := ComputeGraphData(pings, changes, "day", start, end, nil)

	if data.TagColors == nil {
		t.Fatal("TagColors should not be nil")
	}
	for _, tag := range data.AllTags {
		c, ok := data.TagColors[tag]
		if !ok {
			t.Errorf("missing color for tag %q", tag)
			continue
		}
		if len(c) != 7 || c[0] != '#' {
			t.Errorf("tag %q color = %q, want hex like #RRGGBB", tag, c)
		}
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
