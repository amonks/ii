package node

import (
	"fmt"
	"math"
)

// TileBBox returns the bounding box for a Web Mercator tile.
// Returns (south, north, west, east) in degrees.
func TileBBox(z, x, y int) (south, north, west, east float64, err error) {
	if z < 0 || z > 22 {
		return 0, 0, 0, 0, fmt.Errorf("invalid tile coordinates z=%d x=%d y=%d", z, x, y)
	}
	n := 1 << z
	if x < 0 || x >= n || y < 0 || y >= n {
		return 0, 0, 0, 0, fmt.Errorf("invalid tile coordinates z=%d x=%d y=%d", z, x, y)
	}
	north = tileLatDeg(y, n)
	south = tileLatDeg(y+1, n)
	west = tileLonDeg(x, n)
	east = tileLonDeg(x+1, n)
	return south, north, west, east, nil
}

// SignificanceThreshold returns the minimum significance for a tile at zoom z,
// scaled by a detail coefficient.
//
// The threshold drops by 2x per zoom level (linear in tile size), not 4x
// (quadratic). This is correct because a GPS track is a 1D line: the number
// of track points in a tile is proportional to tile width, not tile area.
// With a 2x drop, the number of points surviving per tile stays roughly
// constant across zoom levels, giving consistent visual smoothness.
//
// The detail parameter (0–10) scales the threshold logarithmically:
// threshold = base * 10^(-detail), where base = tileWidth / 256.
func SignificanceThreshold(z int, detail float64) float64 {
	tileSize := 360.0 / math.Pow(2, float64(z))
	base := tileSize / 256.0
	return base * math.Pow(10, -detail)
}

func tileLonDeg(x, n int) float64 {
	return float64(x)/float64(n)*360.0 - 180.0
}

func tileLatDeg(y, n int) float64 {
	latRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y)/float64(n))))
	return latRad * 180.0 / math.Pi
}
