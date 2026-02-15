package logs

import (
	"net/http"
	"time"
)

type TimeRange struct {
	Start string // YYYY-MM-DD, for populating form fields
	End   string
	Range string // e.g. "7d", empty if custom dates

	start time.Time
	end   time.Time
}

var cannedRanges = []struct {
	Key   string
	Label string
	Days  int
}{
	{"24h", "24h", 1},
	{"7d", "7d", 7},
	{"30d", "30d", 30},
	{"90d", "90d", 90},
	{"1y", "1y", 365},
}

func ParseTimeRange(req *http.Request) TimeRange {
	now := time.Now()

	if s := req.URL.Query().Get("start"); s != "" {
		if e := req.URL.Query().Get("end"); e != "" {
			start, err1 := time.Parse("2006-01-02", s)
			end, err2 := time.Parse("2006-01-02", e)
			if err1 == nil && err2 == nil {
				end = end.Add(24*time.Hour - time.Nanosecond)
				return TimeRange{Start: s, End: e, start: start, end: end}
			}
		}
	}

	today := now.Truncate(24 * time.Hour)
	tomorrow := today.Add(24*time.Hour - time.Nanosecond)

	if r := req.URL.Query().Get("range"); r != "" {
		for _, c := range cannedRanges {
			if c.Key == r {
				start := today.AddDate(0, 0, -c.Days)
				return TimeRange{
					Range: r,
					Start: start.Format("2006-01-02"),
					End:   today.Format("2006-01-02"),
					start: start,
					end:   tomorrow,
				}
			}
		}
	}

	// Default: last 7 days.
	start := today.AddDate(0, 0, -7)
	return TimeRange{
		Range: "7d",
		Start: start.Format("2006-01-02"),
		End:   today.Format("2006-01-02"),
		start: start,
		end:   tomorrow,
	}
}

func (tr TimeRange) Days() int {
	d := int(tr.end.Sub(tr.start).Hours() / 24)
	if d < 1 {
		return 1
	}
	return d
}

func (tr TimeRange) StartTime() time.Time { return tr.start }
func (tr TimeRange) EndTime() time.Time   { return tr.end }

type CannedRange struct {
	Key    string
	Label  string
	Active bool
}

func (tr TimeRange) CannedRanges() []CannedRange {
	out := make([]CannedRange, len(cannedRanges))
	for i, c := range cannedRanges {
		out[i] = CannedRange{Key: c.Key, Label: c.Label, Active: tr.Range == c.Key}
	}
	return out
}
