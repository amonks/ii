package node

import (
	"math"
	"math/rand/v2"
	"sort"
	"time"
)

// ScheduleParams defines the parameters for a Poisson schedule.
type ScheduleParams struct {
	Seed       uint64 `json:"seed"`
	PeriodSecs int64  `json:"period_secs"`
}

// PeriodChange represents an event-sourced change to the schedule parameters.
// The schedule uses these parameters starting at Timestamp.
type PeriodChange struct {
	Timestamp  int64  `json:"timestamp"` // unix seconds when this takes effect
	Seed       uint64 `json:"seed"`
	PeriodSecs int64  `json:"period_secs"`
}

// PingsInRange returns all deterministic ping timestamps in [start, end).
// Changes must be sorted by Timestamp and the first must have Timestamp <= start.Unix().
func PingsInRange(changes []PeriodChange, start, end time.Time) []time.Time {
	if !start.Before(end) || len(changes) == 0 {
		return nil
	}

	var pings []time.Time
	startUnix := start.Unix()
	endUnix := end.Unix()

	// Walk through period change segments.
	for i, change := range changes {
		segStart := change.Timestamp
		var segEnd int64
		if i+1 < len(changes) {
			segEnd = changes[i+1].Timestamp
		} else {
			segEnd = endUnix
		}

		// Skip segments entirely before our range.
		if segEnd <= startUnix {
			continue
		}
		// Stop if segment starts at or after our end.
		if segStart >= endUnix {
			break
		}

		// Generate pings for this segment.
		rng := rand.New(rand.NewPCG(change.Seed, 0))
		period := float64(change.PeriodSecs)

		// Walk from segment start to find pings.
		t := float64(segStart)
		for {
			gap := -period * math.Log1p(-rng.Float64())
			t += gap
			ts := int64(t)

			if ts >= segEnd || ts >= endUnix {
				break
			}
			if ts >= startUnix {
				pings = append(pings, time.Unix(ts, 0).UTC())
			}
		}
	}

	sort.Slice(pings, func(i, j int) bool { return pings[i].Before(pings[j]) })
	return pings
}

// NextPing returns the first ping strictly after the given time.
func NextPing(changes []PeriodChange, after time.Time) time.Time {
	// Search forward in small windows until we find one.
	window := 24 * time.Hour
	for attempt := 0; attempt < 100; attempt++ {
		start := after.Add(time.Second) // strictly after
		end := after.Add(window)
		pings := PingsInRange(changes, start, end)
		if len(pings) > 0 {
			return pings[0]
		}
		after = end
		window *= 2
	}
	// Should never happen with reasonable parameters.
	return time.Time{}
}

// PrevPing returns the last ping strictly before the given time.
func PrevPing(changes []PeriodChange, before time.Time) time.Time {
	// Search backward in expanding windows.
	window := 24 * time.Hour
	for attempt := 0; attempt < 100; attempt++ {
		start := before.Add(-window)
		pings := PingsInRange(changes, start, before)
		if len(pings) > 0 {
			return pings[len(pings)-1]
		}
		window *= 2
	}
	return time.Time{}
}
