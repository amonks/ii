package node

import (
	"sort"
	"time"

	"monks.co/pkg/color"
)

// TagSummary is a single tag's aggregated data for a time range.
type TagSummary struct {
	Name      string    `json:"name"`
	TotalSecs float64   `json:"total_secs"`
	Color     string    `json:"color"`
	Sparkline []float64 `json:"sparkline"`
}

// TagSummaryResponse is the JSON response for /tags/summary.
type TagSummaryResponse struct {
	Tags      []TagSummary   `json:"tags"`
	Tree      []*TagTreeNode `json:"tree"`
	TotalSecs float64        `json:"total_secs"`
	Start     time.Time      `json:"start"`
	End       time.Time      `json:"end"`
}

// ComputeTagSummary computes per-tag time totals and sparklines for a time range.
// Each ping's time value is the effective period_secs at that ping's time.
// pingTags maps ping timestamp → tag names; if nil, tags are extracted from blurbs.
func ComputeTagSummary(pings []Ping, changes []PeriodChange, start, end time.Time, pingTags map[int64][]string, numBuckets int) TagSummaryResponse {
	if numBuckets < 1 {
		numBuckets = 20
	}

	bucketDur := end.Sub(start) / time.Duration(numBuckets)
	if bucketDur <= 0 {
		return TagSummaryResponse{Tags: []TagSummary{}, Start: start, End: end}
	}

	type tagData struct {
		totalSecs float64
		sparkline []float64
	}
	tags := make(map[string]*tagData)

	for _, p := range pings {
		if p.Blurb == "" {
			continue
		}

		var tagNames []string
		if pingTags != nil {
			tagNames = pingTags[p.Timestamp]
		} else {
			tagNames = ExtractTags(p.Blurb)
		}
		if len(tagNames) == 0 {
			tagNames = []string{"untagged"}
		}

		period := float64(effectivePeriodAt(changes, p.Timestamp))
		pingTime := time.Unix(p.Timestamp, 0).UTC()

		// Determine sparkline bucket index.
		bucketIdx := max(int(pingTime.Sub(start)/bucketDur), 0)
		if bucketIdx >= numBuckets {
			bucketIdx = numBuckets - 1
		}

		for _, name := range tagNames {
			td, ok := tags[name]
			if !ok {
				td = &tagData{sparkline: make([]float64, numBuckets)}
				tags[name] = td
			}
			td.totalSecs += period
			td.sparkline[bucketIdx] += period
		}
	}

	result := make([]TagSummary, 0, len(tags))
	var totalSecs float64
	for name, td := range tags {
		result = append(result, TagSummary{
			Name:      name,
			TotalSecs: td.totalSecs,
			Color:     color.Hash(name),
			Sparkline: td.sparkline,
		})
		totalSecs += td.totalSecs
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalSecs != result[j].TotalSecs {
			return result[i].TotalSecs > result[j].TotalSecs
		}
		return result[i].Name < result[j].Name
	})

	return TagSummaryResponse{
		Tags:      result,
		TotalSecs: totalSecs,
		Start:     start,
		End:       end,
	}
}
