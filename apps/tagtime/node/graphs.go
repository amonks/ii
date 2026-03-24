package node

import (
	"time"

	"monks.co/pkg/color"
)

// TagBucket holds time-by-tag data for a single time bucket.
type TagBucket struct {
	Start time.Time          `json:"start"`
	End   time.Time          `json:"end"`
	Tags  map[string]float64 `json:"tags"` // tag -> percentage (0-100)
}

// GraphData is the JSON response for /graphs/data.
type GraphData struct {
	Buckets   []TagBucket       `json:"buckets"`
	AllTags   []string          `json:"all_tags"`
	TagColors map[string]string `json:"tag_colors"`
	Window    string            `json:"window"`
}

// ComputeGraphData computes tag time distribution over buckets.
// Each ping's time value is the effective period at that ping's time
// (i.e., the average gap, since each ping represents that much time).
// pingTags maps ping timestamp → tag names from the ping_tags table.
// If nil, tags are extracted from blurbs (legacy behavior).
func ComputeGraphData(pings []Ping, changes []PeriodChange, window string, start, end time.Time, pingTags map[int64][]string) GraphData {
	bucketDur := parseBucketDuration(window)
	if bucketDur == 0 {
		bucketDur = 24 * time.Hour
	}

	// Build buckets.
	var buckets []TagBucket
	for t := start; t.Before(end); t = t.Add(bucketDur) {
		buckets = append(buckets, TagBucket{
			Start: t,
			End:   t.Add(bucketDur),
			Tags:  make(map[string]float64),
		})
	}

	allTagsSet := make(map[string]bool)

	for _, p := range pings {
		if p.Blurb == "" {
			continue
		}
		var tags []string
		if pingTags != nil {
			tags = pingTags[p.Timestamp]
		} else {
			tags = ExtractTags(p.Blurb)
		}
		if len(tags) == 0 {
			tags = []string{"untagged"}
		}

		pingTime := time.Unix(p.Timestamp, 0).UTC()
		period := effectivePeriodAt(changes, p.Timestamp)

		// Find which bucket this ping belongs to.
		for i := range buckets {
			if !pingTime.Before(buckets[i].Start) && pingTime.Before(buckets[i].End) {
				for _, tag := range tags {
					buckets[i].Tags[tag] += float64(period)
					allTagsSet[tag] = true
				}
				break
			}
		}
	}

	// Normalize each bucket's tags to percentages (0-100).
	for i := range buckets {
		var total float64
		for _, v := range buckets[i].Tags {
			total += v
		}
		if total > 0 {
			for tag, v := range buckets[i].Tags {
				buckets[i].Tags[tag] = (v / total) * 100
			}
		}
	}

	var allTags []string
	tagColors := make(map[string]string)
	for tag := range allTagsSet {
		allTags = append(allTags, tag)
		tagColors[tag] = color.Hash(tag)
	}

	return GraphData{
		Buckets:   buckets,
		AllTags:   allTags,
		TagColors: tagColors,
		Window:    window,
	}
}

// effectivePeriodAt returns the period_secs active at a given unix timestamp.
func effectivePeriodAt(changes []PeriodChange, ts int64) int64 {
	period := int64(2700) // default 45 min
	for _, c := range changes {
		if c.Timestamp <= ts {
			period = c.PeriodSecs
		} else {
			break
		}
	}
	return period
}

func parseBucketDuration(window string) time.Duration {
	switch window {
	case "hour":
		return time.Hour
	case "day":
		return 24 * time.Hour
	case "week":
		return 7 * 24 * time.Hour
	case "month":
		return 30 * 24 * time.Hour
	case "year":
		return 365 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}
