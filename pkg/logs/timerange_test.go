package logs

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestParseTimeRange_RFC3339(t *testing.T) {
	start := time.Date(2026, 3, 1, 10, 30, 0, 0, time.UTC)
	end := time.Date(2026, 3, 1, 11, 45, 0, 0, time.UTC)

	req := &http.Request{URL: &url.URL{
		RawQuery: url.Values{
			"start": {start.Format(time.RFC3339)},
			"end":   {end.Format(time.RFC3339)},
		}.Encode(),
	}}

	tr := ParseTimeRange(req)

	if !tr.StartTime().Equal(start) {
		t.Errorf("start = %v, want %v", tr.StartTime(), start)
	}
	if !tr.EndTime().Equal(end) {
		t.Errorf("end = %v, want %v", tr.EndTime(), end)
	}
	// Range should be empty (custom dates, not a canned range).
	if tr.Range != "" {
		t.Errorf("Range = %q, want empty", tr.Range)
	}
}

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
