package node

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"time"
)

// backfillSubscriptions fetches tiles from upstream to populate subscription
// areas. For each subscription, it computes the zoom level whose significance
// threshold matches min_significance, then fetches all tiles covering the
// subscription's bbox at that zoom level.
func backfillSubscriptions(ctx context.Context, store *Store, config Config) {
	if config.Upstream == "" {
		return
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, sub := range config.Subscriptions {
		z := zoomForSignificance(sub.MinSignificance)
		west, south, east, north := sub.BBox[0], sub.BBox[1], sub.BBox[2], sub.BBox[3]

		minX, maxX := lonToTileX(west, z), lonToTileX(east, z)
		minY, maxY := latToTileY(north, z), latToTileY(south, z) // note: y is inverted

		slog.Info("backfilling subscription",
			"bbox", sub.BBox,
			"zoom", z,
			"tiles", (maxX-minX+1)*(maxY-minY+1),
		)

		for x := minX; x <= maxX; x++ {
			for y := minY; y <= maxY; y++ {
				if ctx.Err() != nil {
					return
				}
				points, err := FetchTileFromUpstream(ctx, client, config.Upstream, z, x, y)
				if err != nil {
					slog.Warn("backfill tile fetch failed",
						"z", z, "x", x, "y", y,
						"error", err,
					)
					continue
				}
				if len(points) > 0 {
					WriteUpstreamPoints(ctx, store, points, config.Subscriptions)
				}
			}
		}
	}
}

// zoomForSignificance returns the zoom level whose significance threshold
// is closest to (but not greater than) the given min_significance.
// If min_significance is 0, returns zoom 22 (maximum detail).
func zoomForSignificance(minSig float64) int {
	if minSig <= 0 {
		return 22
	}
	// SignificanceThreshold(z) = (360 / 2^z)^2 / 256^2
	// Solve for z: 2^z = 360 / (256 * sqrt(minSig))
	// z = log2(360 / (256 * sqrt(minSig)))
	z := math.Log2(360.0 / (256.0 * math.Sqrt(minSig)))
	zi := int(math.Ceil(z))
	if zi < 0 {
		return 0
	}
	if zi > 22 {
		return 22
	}
	return zi
}

// lonToTileX converts a longitude to a tile X coordinate at the given zoom.
func lonToTileX(lon float64, z int) int {
	n := float64(int(1) << z)
	x := int((lon + 180.0) / 360.0 * n)
	if x < 0 {
		x = 0
	}
	if x >= int(n) {
		x = int(n) - 1
	}
	return x
}

// latToTileY converts a latitude to a tile Y coordinate at the given zoom.
func latToTileY(lat float64, z int) int {
	n := float64(int(1) << z)
	latRad := lat * math.Pi / 180.0
	y := int((1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n)
	if y < 0 {
		y = 0
	}
	if y >= int(n) {
		y = int(n) - 1
	}
	return y
}
