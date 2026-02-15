package logs

import (
	"testing"
	"time"
)

func TestWindowMs(t *testing.T) {
	now := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		days   int
		wantMs int
	}{
		{"24h uses hourly windows", 1, 3600000},
		{"3d uses hourly windows", 3, 3600000},
		{"6d uses hourly windows", 6, 3600000},
		{"7d uses daily windows", 7, 86400000},
		{"30d uses daily windows", 30, 86400000},
		{"90d uses daily windows", 90, 86400000},
		{"180d uses weekly windows", 180, 604800000},
		{"365d uses weekly windows", 365, 604800000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := TimeRange{
				start: now.AddDate(0, 0, -tt.days),
				end:   now,
			}
			got := tr.WindowMs()
			if got != tt.wantMs {
				t.Errorf("WindowMs() for %d days = %d, want %d", tt.days, got, tt.wantMs)
			}
		})
	}
}
