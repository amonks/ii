package ui

import (
	"testing"
	"time"
)

func TestFormatDurationShort(t *testing.T) {
	cases := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{name: "seconds", duration: 45 * time.Second, want: "45s"},
		{name: "minutes", duration: 2*time.Minute + 10*time.Second, want: "2m"},
		{name: "hours", duration: 3*time.Hour + 5*time.Minute, want: "3h"},
		{name: "days", duration: 48 * time.Hour, want: "2d"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatDurationShort(tc.duration)
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func TestFormatTimeAgo(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	then := now.Add(-2 * time.Minute)

	got := FormatTimeAgo(then, now)
	if got != "2m ago" {
		t.Fatalf("expected 2m ago, got %s", got)
	}
}

func TestFormatTimeAgeShort(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	then := now.Add(-2 * time.Minute)

	got := FormatTimeAgeShort(then, now)
	if got != "2m" {
		t.Fatalf("expected 2m, got %s", got)
	}
}

func TestFormatTimeAgoClampsFutureTime(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	then := now.Add(5 * time.Second)

	got := FormatTimeAgo(then, now)
	if got != "0s ago" {
		t.Fatalf("expected 0s ago, got %s", got)
	}
}
