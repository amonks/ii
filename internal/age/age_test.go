package age

import (
	"testing"
	"time"
)

func TestDurationData(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-10 * time.Minute)
	completed := start.Add(3 * time.Minute)
	futureStart := now.Add(4 * time.Minute)
	pastCompleted := now.Add(-2 * time.Minute)

	cases := []struct {
		name            string
		startedAt       time.Time
		completedAt     time.Time
		durationSeconds int
		active          bool
		want            time.Duration
		ok              bool
	}{
		{
			name:      "active uses now",
			startedAt: start,
			active:    true,
			want:      10 * time.Minute,
			ok:        true,
		},
		{
			name:      "active zero duration",
			startedAt: now,
			active:    true,
			want:      0,
			ok:        true,
		},
		{
			name:      "active clamps future",
			startedAt: futureStart,
			active:    true,
			want:      0,
			ok:        true,
		},
		{
			name:            "duration seconds preferred",
			startedAt:       start,
			completedAt:     now,
			durationSeconds: 90,
			want:            90 * time.Second,
			ok:              true,
		},
		{
			name:        "completed uses timestamps",
			startedAt:   start,
			completedAt: completed,
			want:        3 * time.Minute,
			ok:          true,
		},
		{
			name:        "completed clamps negative",
			startedAt:   now,
			completedAt: pastCompleted,
			want:        0,
			ok:          true,
		},
		{
			name:   "missing timing data",
			active: true,
			want:   0,
			ok:     false,
		},
		{
			name:      "completed missing timestamps",
			startedAt: start,
			want:      0,
			ok:        false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := DurationData(tc.startedAt, tc.completedAt, tc.durationSeconds, tc.active, now)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("expected %s/%t, got %s/%t", tc.want, tc.ok, got, ok)
			}
		})
	}
}

func TestAgeData(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	started := now.Add(-4 * time.Minute)
	future := now.Add(2 * time.Minute)

	cases := []struct {
		name      string
		startedAt time.Time
		want      time.Duration
		ok        bool
	}{
		{
			name:      "uses started time",
			startedAt: started,
			want:      4 * time.Minute,
			ok:        true,
		},
		{
			name:      "clamps future start",
			startedAt: future,
			want:      0,
			ok:        true,
		},
		{
			name:      "missing started time",
			startedAt: time.Time{},
			want:      0,
			ok:        false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := AgeData(tc.startedAt, now)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("expected %s/%t, got %s/%t", tc.want, tc.ok, got, ok)
			}
		})
	}
}
