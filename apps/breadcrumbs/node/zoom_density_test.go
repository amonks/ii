package node

import (
	"context"
	"math"
	"testing"

	pb "monks.co/apps/breadcrumbs/proto"
)

// generateGPSTrack creates a realistic GPS track of n points walking along a
// street in a ~0.01° lat × 0.01° lon area (roughly 1km × 1km). Points have
// small lateral jitter to produce nonzero VW triangle areas.
func generateGPSTrack(n int, baseLat, baseLon float64) []*pb.Point {
	points := make([]*pb.Point, n)
	for i := 0; i < n; i++ {
		frac := float64(i) / float64(n-1)
		// Walk northeast with small sinusoidal lateral wobble.
		lat := baseLat + frac*0.01
		lon := baseLon + frac*0.01 + math.Sin(float64(i)*0.3)*0.0001
		points[i] = &pb.Point{
			Timestamp: int64(i + 1),
			Latitude:  lat,
			Longitude: lon,
		}
	}
	return points
}

// ingestTrack runs points through the simplifier and inserts them into the
// store, just like the real ingest handler does.
func ingestTrack(t *testing.T, s *Store, simp *Simplifier, points []*pb.Point) {
	t.Helper()
	ctx := context.Background()
	for _, p := range points {
		result := simp.Append(p)
		if err := s.InsertPoint(ctx, p, result.NewPointSig, false); err != nil {
			t.Fatal(err)
		}
		if result.HasPrevUpdate {
			if err := s.UpdateSignificance(ctx, result.PrevTailTimestamp, result.PrevTailSig); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// tileContaining returns the tile coordinates (z, x, y) for the tile that
// contains the given lat/lon at zoom level z.
func tileContaining(z int, lat, lon float64) (x, y int) {
	n := math.Pow(2, float64(z))
	x = int((lon + 180.0) / 360.0 * n)
	latRad := lat * math.Pi / 180.0
	y = int((1 - math.Log(math.Tan(latRad)+1/math.Cos(latRad))/math.Pi) / 2 * n)
	if x < 0 {
		x = 0
	}
	if x >= int(n) {
		x = int(n) - 1
	}
	if y < 0 {
		y = 0
	}
	if y >= int(n) {
		y = int(n) - 1
	}
	return x, y
}

// TestSignificanceThresholdDropsPerZoom verifies that the threshold drops by
// 4x per zoom level (quadratic in tile size), matching the area-based
// significance metric from Visvalingam-Whyatt. This holds for any fixed
// detail value.
func TestSignificanceThresholdDropsPerZoom(t *testing.T) {
	detail := 5.0

	t.Logf("%-5s %-20s %-10s", "Zoom", "Threshold", "Ratio")
	prev := SignificanceThreshold(0, detail)
	t.Logf("%-5d %-20e %-10s", 0, prev, "-")

	for z := 1; z <= 18; z++ {
		threshold := SignificanceThreshold(z, detail)
		ratio := prev / threshold
		t.Logf("%-5d %-20e %-10.2f", z, threshold, ratio)

		// Each zoom level should reduce threshold by 4x (area-based).
		if math.Abs(ratio-4.0) > 0.01 {
			t.Errorf("z=%d: ratio = %.2f, want 4.0", z, ratio)
		}
		prev = threshold
	}
}

// TestPointDensityPerTileAtFixedDetail measures how many points survive
// per tile at different zoom levels for a realistic track.
func TestPointDensityPerTileAtFixedDetail(t *testing.T) {
	s := testStore(t)
	simp := NewSimplifier()

	// Generate a dense track: ~1000 points over ~0.01° (~1km).
	points := generateGPSTrack(1000, 41.88, -87.63)
	ingestTrack(t, s, simp, points)

	ctx := context.Background()
	detail := 5.0

	centerLat := 41.88 + 0.005
	centerLon := -87.63 + 0.005

	t.Logf("%-5s %-8s %-20s %-20s", "Zoom", "Points", "Threshold", "Tile Area (sq°)")

	for z := 10; z <= 20; z++ {
		x, y := tileContaining(z, centerLat, centerLon)
		south, north, west, east, err := TileBBox(z, x, y)
		if err != nil {
			t.Fatalf("z=%d: %v", z, err)
		}

		minSig := SignificanceThreshold(z, detail)

		latBuf := (north - south) * 0.1
		lonBuf := (east - west) * 0.1
		pts, err := s.QueryTile(ctx,
			south-latBuf, north+latBuf, west-lonBuf, east+lonBuf, minSig)
		if err != nil {
			t.Fatalf("z=%d: %v", z, err)
		}

		area := (north - south) * (east - west)
		t.Logf("%-5d %-8d %-20e %-20e", z, len(pts), minSig, area)
	}
}

// TestVWSignificanceDistribution shows the distribution of significance
// values from online VW to understand what threshold values are meaningful.
func TestVWSignificanceDistribution(t *testing.T) {
	simp := NewSimplifier()
	points := generateGPSTrack(1000, 41.88, -87.63)

	var sigs []float64
	for _, p := range points {
		result := simp.Append(p)
		if result.HasPrevUpdate {
			sigs = append(sigs, result.PrevTailSig)
		}
	}

	if len(sigs) == 0 {
		t.Fatal("no significance values computed")
	}

	// Sort and compute percentiles.
	for i := 1; i < len(sigs); i++ {
		for j := i; j > 0 && sigs[j] < sigs[j-1]; j-- {
			sigs[j], sigs[j-1] = sigs[j-1], sigs[j]
		}
	}

	t.Logf("Significance distribution for %d points (0.01° track):", len(sigs))
	t.Logf("  min:  %e", sigs[0])
	t.Logf("  p25:  %e", sigs[len(sigs)/4])
	t.Logf("  p50:  %e", sigs[len(sigs)/2])
	t.Logf("  p75:  %e", sigs[len(sigs)*3/4])
	t.Logf("  max:  %e", sigs[len(sigs)-1])

	// Compare with thresholds at various zoom levels (detail=0, the base case).
	t.Logf("\nThresholds vs significance percentiles (detail=0):")
	t.Logf("%-5s %-20s %-20s", "Zoom", "Threshold", "% points surviving")
	for z := 0; z <= 22; z++ {
		threshold := SignificanceThreshold(z, 0)
		surviving := 0
		for _, sig := range sigs {
			if sig >= threshold {
				surviving++
			}
		}
		total := len(sigs) + 2
		surviving += 2 // first and last always survive (MaxFloat64)
		pct := float64(surviving) / float64(total) * 100
		t.Logf("%-5d %-20e %-10.1f%%", z, threshold, pct)
	}
}

// TestConstantVisualDensity verifies that the number of points per tile
// stays roughly constant across zoom levels, giving consistent visual
// smoothness when zooming in. This is the key behavioral test.
func TestConstantVisualDensity(t *testing.T) {
	s := testStore(t)
	simp := NewSimplifier()

	points := generateGPSTrack(1000, 41.88, -87.63)
	ingestTrack(t, s, simp, points)

	ctx := context.Background()
	detail := 5.0

	centerLat := 41.88 + 0.005
	centerLon := -87.63 + 0.005

	// Query point counts at consecutive zoom levels where the track
	// is visible. The track spans ~0.01°, so it first appears around z≈15
	// and is fully contained by z≈18.
	type result struct {
		z     int
		count int
	}
	var results []result

	for z := 16; z <= 20; z++ {
		x, y := tileContaining(z, centerLat, centerLon)
		south, north, west, east, err := TileBBox(z, x, y)
		if err != nil {
			t.Fatalf("z=%d: %v", z, err)
		}

		minSig := SignificanceThreshold(z, detail)
		latBuf := (north - south) * 0.1
		lonBuf := (east - west) * 0.1
		pts, err := s.QueryTile(ctx,
			south-latBuf, north+latBuf, west-lonBuf, east+lonBuf, minSig)
		if err != nil {
			t.Fatalf("z=%d: %v", z, err)
		}
		t.Logf("z=%d: %d points, threshold=%e", z, len(pts), minSig)
		results = append(results, result{z, len(pts)})
	}

	// Find two consecutive zoom levels where both have points. The count
	// should not explode — if threshold is correct, zooming in shows more
	// detail but per-tile count stays bounded.
	for i := 1; i < len(results); i++ {
		lo := results[i-1]
		hi := results[i]
		if lo.count > 0 && hi.count > 0 {
			// At higher zoom, the tile is smaller (fewer points spatially)
			// but threshold is lower (more points survive). These should
			// roughly cancel out. Allow up to 4x growth.
			if hi.count > lo.count*4 {
				t.Errorf("z=%d→%d: point count jumped from %d to %d (>4x); "+
					"threshold may be dropping too fast",
					lo.z, hi.z, lo.count, hi.count)
			}
		}
	}

	// At z=20 (very high zoom), nearly all points in the tile should survive.
	last := results[len(results)-1]
	if last.count < 10 {
		t.Errorf("z=%d: only %d points survived; expected many at high zoom", last.z, last.count)
	}
}

// TestDistanceFloorSurvivesAtLowZoom verifies that the distance_floor method
// lets points survive at much lower zoom levels than pure area, because
// collinear points along long segments get significance from neighbor distance.
func TestDistanceFloorSurvivesAtLowZoom(t *testing.T) {
	s := testStore(t)
	simp := NewSimplifier()

	points := generateGPSTrack(1000, 41.88, -87.63)
	ingestTrack(t, s, simp, points)

	ctx := context.Background()
	detail := 5.0

	// With pure area, almost nothing survives at z=10 (threshold ≈ 1.9e-11,
	// most significance values are smaller).
	areaCount := 0
	{
		x, y := tileContaining(10, 41.885, -87.625)
		south, north, west, east, _ := TileBBox(10, x, y)
		minSig := SignificanceThreshold(10, detail)
		latBuf := (north - south) * 0.1
		lonBuf := (east - west) * 0.1
		pts, _ := s.QueryTile(ctx, south-latBuf, north+latBuf, west-lonBuf, east+lonBuf, minSig)
		areaCount = len(pts)
		t.Logf("area method z=10: %d points (threshold=%e)", areaCount, minSig)
	}

	// Recompute with distance_floor.
	n, err := s.RecomputeSignificance(ctx, MethodDistanceFloor)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("recomputed %d points with distance_floor", n)

	// Now query again — distance_floor should let many more points survive.
	floorCount := 0
	{
		x, y := tileContaining(10, 41.885, -87.625)
		south, north, west, east, _ := TileBBox(10, x, y)
		minSig := SignificanceThreshold(10, detail)
		latBuf := (north - south) * 0.1
		lonBuf := (east - west) * 0.1
		pts, _ := s.QueryTile(ctx, south-latBuf, north+latBuf, west-lonBuf, east+lonBuf, minSig)
		floorCount = len(pts)
		t.Logf("distance_floor method z=10: %d points (threshold=%e)", floorCount, minSig)
	}

	if floorCount <= areaCount {
		t.Errorf("distance_floor (%d) should show more points than area (%d) at z=10",
			floorCount, areaCount)
	}
}
