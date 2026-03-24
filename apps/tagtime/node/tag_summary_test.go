package node

import (
	"math"
	"testing"
	"time"
)

func TestComputeTagSummaryOrdering(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "#code"},
		{Timestamp: start.Add(2 * time.Hour).Unix(), Blurb: "#code"},
		{Timestamp: start.Add(3 * time.Hour).Unix(), Blurb: "#code"},
		{Timestamp: start.Add(4 * time.Hour).Unix(), Blurb: "#sleep"},
	}

	resp := ComputeTagSummary(pings, changes, start, end, nil, 10)

	if len(resp.Tags) != 2 {
		t.Fatalf("got %d tags, want 2", len(resp.Tags))
	}
	// code has 3 pings * 2700 = 8100, sleep has 1 * 2700 = 2700
	if resp.Tags[0].Name != "code" {
		t.Errorf("first tag = %q, want code", resp.Tags[0].Name)
	}
	if resp.Tags[1].Name != "sleep" {
		t.Errorf("second tag = %q, want sleep", resp.Tags[1].Name)
	}
	if math.Abs(resp.Tags[0].TotalSecs-8100) > 0.1 {
		t.Errorf("code total_secs = %.1f, want 8100", resp.Tags[0].TotalSecs)
	}
	if math.Abs(resp.Tags[1].TotalSecs-2700) > 0.1 {
		t.Errorf("sleep total_secs = %.1f, want 2700", resp.Tags[1].TotalSecs)
	}
	if math.Abs(resp.TotalSecs-10800) > 0.1 {
		t.Errorf("total_secs = %.1f, want 10800", resp.TotalSecs)
	}
}

func TestComputeTagSummarySparkline(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC) // 10 hours
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}

	pings := []Ping{
		// All in the first half of the range
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "#code"},
		{Timestamp: start.Add(2 * time.Hour).Unix(), Blurb: "#code"},
	}

	resp := ComputeTagSummary(pings, changes, start, end, nil, 10)

	if len(resp.Tags) != 1 {
		t.Fatalf("got %d tags, want 1", len(resp.Tags))
	}
	sparkline := resp.Tags[0].Sparkline
	if len(sparkline) != 10 {
		t.Fatalf("sparkline len = %d, want 10", len(sparkline))
	}
	// 10 buckets of 1 hour each. Pings at hour 1 and 2 → buckets [1] and [2].
	if sparkline[0] != 0 {
		t.Errorf("sparkline[0] = %.1f, want 0", sparkline[0])
	}
	if sparkline[1] != 2700 {
		t.Errorf("sparkline[1] = %.1f, want 2700", sparkline[1])
	}
	if sparkline[2] != 2700 {
		t.Errorf("sparkline[2] = %.1f, want 2700", sparkline[2])
	}
	// Rest should be zero
	for i := 3; i < 10; i++ {
		if sparkline[i] != 0 {
			t.Errorf("sparkline[%d] = %.1f, want 0", i, sparkline[i])
		}
	}
}

func TestComputeTagSummaryPeriodChange(t *testing.T) {
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

	resp := ComputeTagSummary(pings, changes, start, end, nil, 10)

	if len(resp.Tags) != 1 {
		t.Fatalf("got %d tags, want 1", len(resp.Tags))
	}
	// 2700 + 900 = 3600
	if math.Abs(resp.Tags[0].TotalSecs-3600) > 0.1 {
		t.Errorf("code total_secs = %.1f, want 3600", resp.Tags[0].TotalSecs)
	}
}

func TestComputeTagSummaryEmptyBlurb(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: ""},
	}

	resp := ComputeTagSummary(pings, changes, start, end, nil, 10)

	if len(resp.Tags) != 0 {
		t.Errorf("empty blurb should produce no tags, got %d", len(resp.Tags))
	}
}

func TestComputeTagSummaryUntagged(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "just some text no tags"},
	}

	resp := ComputeTagSummary(pings, changes, start, end, nil, 10)

	if len(resp.Tags) != 1 {
		t.Fatalf("got %d tags, want 1 (untagged)", len(resp.Tags))
	}
	if resp.Tags[0].Name != "untagged" {
		t.Errorf("tag name = %q, want untagged", resp.Tags[0].Name)
	}
}

func TestComputeTagSummaryWithPingTags(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}

	ts1 := start.Add(1 * time.Hour).Unix()
	ts2 := start.Add(2 * time.Hour).Unix()
	pings := []Ping{
		{Timestamp: ts1, Blurb: "#old"},
		{Timestamp: ts2, Blurb: "#work"},
	}
	// pingTags overrides blurb extraction (simulates post-rename state)
	pingTags := map[int64][]string{
		ts1: {"new"},  // renamed from "old" to "new"
		ts2: {"work"},
	}

	resp := ComputeTagSummary(pings, changes, start, end, pingTags, 10)

	if len(resp.Tags) != 2 {
		t.Fatalf("got %d tags, want 2", len(resp.Tags))
	}
	// Both have equal time, so order is by total_secs (same) then stable sort
	names := map[string]bool{}
	for _, tag := range resp.Tags {
		names[tag.Name] = true
	}
	if !names["new"] {
		t.Error("expected tag 'new' from pingTags")
	}
	if !names["work"] {
		t.Error("expected tag 'work' from pingTags")
	}
}

func TestComputeTagSummaryColors(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "#code #meeting"},
	}

	resp := ComputeTagSummary(pings, changes, start, end, nil, 10)

	for _, tag := range resp.Tags {
		if len(tag.Color) != 7 || tag.Color[0] != '#' {
			t.Errorf("tag %q color = %q, want hex like #RRGGBB", tag.Name, tag.Color)
		}
	}
}

func TestComputeTagSummaryMultiTagPing(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}

	pings := []Ping{
		{Timestamp: start.Add(1 * time.Hour).Unix(), Blurb: "#code #meeting"},
	}

	resp := ComputeTagSummary(pings, changes, start, end, nil, 10)

	// Both tags get the full period_secs (same as graphs behavior)
	if len(resp.Tags) != 2 {
		t.Fatalf("got %d tags, want 2", len(resp.Tags))
	}
	for _, tag := range resp.Tags {
		if math.Abs(tag.TotalSecs-2700) > 0.1 {
			t.Errorf("tag %q total_secs = %.1f, want 2700", tag.Name, tag.TotalSecs)
		}
	}
}
