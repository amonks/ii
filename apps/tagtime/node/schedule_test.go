package node

import (
	"math"
	"testing"
	"time"
)

func TestScheduleDeterminism(t *testing.T) {
	params := ScheduleParams{Seed: 12345, PeriodSecs: 2700}
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	pings1 := PingsInRange([]PeriodChange{{Timestamp: 0, Seed: params.Seed, PeriodSecs: params.PeriodSecs}}, start, end)
	pings2 := PingsInRange([]PeriodChange{{Timestamp: 0, Seed: params.Seed, PeriodSecs: params.PeriodSecs}}, start, end)

	if len(pings1) == 0 {
		t.Fatal("expected pings, got none")
	}
	if len(pings1) != len(pings2) {
		t.Fatalf("different lengths: %d vs %d", len(pings1), len(pings2))
	}
	for i := range pings1 {
		if pings1[i] != pings2[i] {
			t.Errorf("ping %d differs: %v vs %v", i, pings1[i], pings2[i])
		}
	}
}

func TestScheduleAverageGap(t *testing.T) {
	periodSecs := int64(2700) // 45 minutes
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: periodSecs}}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(365 * 24 * time.Hour) // one year

	pings := PingsInRange(changes, start, end)
	if len(pings) < 100 {
		t.Fatalf("expected many pings over a year, got %d", len(pings))
	}

	totalGap := pings[len(pings)-1].Sub(pings[0]).Seconds()
	avgGap := totalGap / float64(len(pings)-1)
	expected := float64(periodSecs)

	// Should be within 5% of expected for a year of data.
	if math.Abs(avgGap-expected)/expected > 0.05 {
		t.Errorf("average gap = %.1f, want ~%.1f (±5%%)", avgGap, expected)
	}
}

func TestSchedulePingsInRangeEmpty(t *testing.T) {
	changes := []PeriodChange{{Timestamp: 0, Seed: 1, PeriodSecs: 2700}}
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := start // zero-width range

	pings := PingsInRange(changes, start, end)
	if len(pings) != 0 {
		t.Errorf("expected no pings for empty range, got %d", len(pings))
	}
}

func TestScheduleDifferentSeeds(t *testing.T) {
	changes1 := []PeriodChange{{Timestamp: 0, Seed: 111, PeriodSecs: 2700}}
	changes2 := []PeriodChange{{Timestamp: 0, Seed: 222, PeriodSecs: 2700}}
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	pings1 := PingsInRange(changes1, start, end)
	pings2 := PingsInRange(changes2, start, end)

	if len(pings1) == 0 || len(pings2) == 0 {
		t.Fatal("expected pings from both seeds")
	}

	// Different seeds should produce different schedules.
	same := true
	for i := 0; i < min(len(pings1), len(pings2)); i++ {
		if pings1[i] != pings2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds produced identical schedules")
	}
}

func TestSchedulePeriodChange(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

	changes := []PeriodChange{
		{Timestamp: 0, Seed: 42, PeriodSecs: 2700},                   // 45min average
		{Timestamp: mid.Unix(), Seed: 42, PeriodSecs: 900},           // 15min average at noon
	}

	pings := PingsInRange(changes, start, end)
	if len(pings) == 0 {
		t.Fatal("expected pings")
	}

	// Count pings before and after the period change.
	var before, after int
	for _, p := range pings {
		if p.Before(mid) {
			before++
		} else {
			after++
		}
	}

	// With 3x shorter period in second half, we expect roughly 3x more pings.
	if after < before {
		t.Errorf("expected more pings after period decrease: before=%d, after=%d", before, after)
	}
}

func TestNextPing(t *testing.T) {
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}
	after := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	next := NextPing(changes, after)
	if !next.After(after) {
		t.Errorf("NextPing should be after %v, got %v", after, next)
	}

	// Should match the first ping in a range starting from after.
	pings := PingsInRange(changes, after.Add(1), after.Add(24*time.Hour))
	if len(pings) == 0 {
		t.Fatal("expected pings in range")
	}
	if next != pings[0] {
		t.Errorf("NextPing = %v, first ping in range = %v", next, pings[0])
	}
}

func TestPrevPing(t *testing.T) {
	changes := []PeriodChange{{Timestamp: 0, Seed: 42, PeriodSecs: 2700}}
	before := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	prev := PrevPing(changes, before)
	if !prev.Before(before) {
		t.Errorf("PrevPing should be before %v, got %v", before, prev)
	}

	// The next ping after prev should be >= before.
	next := NextPing(changes, prev)
	if next.Before(before) {
		t.Errorf("next ping after PrevPing (%v) should be >= %v, got %v", prev, before, next)
	}
}
