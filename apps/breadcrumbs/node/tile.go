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
// scaled by a detail parameter.
//
// Significance is measured in square degrees (triangle area from
// Visvalingam-Whyatt). The base threshold is the area of one pixel in tile
// coordinates: tile_area / tile_pixels = (360/2^z)^2 / 256^2.
//
// The detail parameter (0–10) scales the threshold logarithmically:
// threshold = base * 10^(-detail). At detail=0 the threshold equals one
// pixel's area; higher detail lowers the threshold, retaining more points.
//
// The base drops by 4x per zoom level. For a 1D GPS track, the number of
// points in a tile is proportional to tile width (halves per zoom), while
// the fraction surviving the lower threshold roughly doubles per zoom,
// so the net points per tile stays approximately constant across zoom
// levels for any fixed detail setting.
func SignificanceThreshold(z int, detail float64) float64 {
	tileSize := 360.0 / math.Pow(2, float64(z))
	pixelSize := tileSize / 256.0
	return pixelSize * pixelSize * math.Pow(10, -detail)
}

// zoomForSigAtDetail returns the zoom level at which a point with the given
// significance first becomes visible at the given detail setting.
func zoomForSigAtDetail(sig, detail float64) int {
	if sig <= 0 {
		return 22
	}
	// SignificanceThreshold(z, detail) = (360 / (256 * 2^z))^2 * 10^(-detail)
	// sig = (360 / (256 * 2^z))^2 * 10^(-detail)
	// (360 / (256 * 2^z))^2 = sig * 10^detail
	// 360 / (256 * 2^z) = sqrt(sig * 10^detail)
	// 2^z = 360 / (256 * sqrt(sig * 10^detail))
	adj := sig * math.Pow(10, detail)
	z := math.Log2(360.0 / (256.0 * math.Sqrt(adj)))
	zi := int(math.Ceil(z))
	if zi < 0 {
		return 0
	}
	if zi > 22 {
		return 22
	}
	return zi
}

func tileLonDeg(x, n int) float64 {
	return float64(x)/float64(n)*360.0 - 180.0
}

func tileLatDeg(y, n int) float64 {
	latRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y)/float64(n))))
	return latRad * 180.0 / math.Pi
}
