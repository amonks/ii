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

// SignificanceThreshold returns the minimum significance for a tile at zoom z.
// Points whose triangle area (in square degrees) is smaller than this are excluded.
func SignificanceThreshold(z int) float64 {
	tileSize := 360.0 / math.Pow(2, float64(z))
	return (tileSize * tileSize) / (256.0 * 256.0)
}

func tileLonDeg(x, n int) float64 {
	return float64(x)/float64(n)*360.0 - 180.0
}

func tileLatDeg(y, n int) float64 {
	latRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y)/float64(n))))
	return latRad * 180.0 / math.Pi
}
