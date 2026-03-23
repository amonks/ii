package node

import (
	"context"

	pb "monks.co/apps/breadcrumbs/proto"
)

// visitGapNanos is the minimum time gap between consecutive points that
// indicates the track left and re-entered the tile area. 120 seconds is
// well above the 1Hz sampling rate and handles significance-filtered gaps
// at low zoom levels.
const visitGapNanos int64 = 120_000_000_000

// splitVisits splits timestamp-ordered points into segments at gaps
// exceeding visitGapNanos. Returns at least one segment if input is non-empty.
func splitVisits(points []*pb.Point) [][]*pb.Point {
	if len(points) == 0 {
		return nil
	}
	var segments [][]*pb.Point
	start := 0
	for i := 1; i < len(points); i++ {
		if points[i].Timestamp-points[i-1].Timestamp > visitGapNanos {
			segments = append(segments, points[start:i])
			start = i
		}
	}
	segments = append(segments, points[start:])
	return segments
}

// neighborFetcher is the subset of Store needed by extendSegments.
type neighborFetcher interface {
	PrevVisiblePoint(ctx context.Context, beforeTimestamp int64, minSig float64) (*pb.Point, error)
	NextVisiblePoint(ctx context.Context, afterTimestamp int64, minSig float64) (*pb.Point, error)
}

// extendSegments prepends/appends temporal neighbor points to each segment
// so that lines extend past tile edges and connect with adjacent tiles.
// It skips extension at the global first/last timestamp (the true track
// endpoints, where no neighbor exists).
func extendSegments(
	ctx context.Context,
	segments [][]*pb.Point,
	minSig float64,
	globalFirst, globalLast int64,
	store neighborFetcher,
) ([][]*pb.Point, error) {
	for i, seg := range segments {
		if len(seg) == 0 {
			continue
		}

		// Prepend predecessor if this isn't the global start.
		firstTS := seg[0].Timestamp
		if firstTS != globalFirst {
			prev, err := store.PrevVisiblePoint(ctx, firstTS, minSig)
			if err != nil {
				return nil, err
			}
			if prev != nil {
				segments[i] = append([]*pb.Point{prev}, seg...)
				seg = segments[i]
			}
		}

		// Append successor if this isn't the global end.
		lastTS := seg[len(seg)-1].Timestamp
		if lastTS != globalLast {
			next, err := store.NextVisiblePoint(ctx, lastTS, minSig)
			if err != nil {
				return nil, err
			}
			if next != nil {
				segments[i] = append(seg, next)
			}
		}
	}
	return segments, nil
}
