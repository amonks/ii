package node

import (
	"math"
	"testing"
)

func TestTileBBoxZoom0(t *testing.T) {
	south, north, west, east, err := TileBBox(0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(west-(-180)) > 0.001 {
		t.Errorf("west = %f, want -180", west)
	}
	if math.Abs(east-180) > 0.001 {
		t.Errorf("east = %f, want 180", east)
	}
	// Web Mercator max latitude ~85.0511
	if math.Abs(north-85.0511) > 0.01 {
		t.Errorf("north = %f, want ~85.0511", north)
	}
	if math.Abs(south-(-85.0511)) > 0.01 {
		t.Errorf("south = %f, want ~-85.0511", south)
	}
}

func TestTileBBoxZoom1(t *testing.T) {
	// z=1: 4 tiles. (0,0) is NW quadrant.
	south, north, west, east, err := TileBBox(1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(west-(-180)) > 0.001 {
		t.Errorf("west = %f", west)
	}
	if math.Abs(east-0) > 0.001 {
		t.Errorf("east = %f", east)
	}
	if north < 80 {
		t.Errorf("north = %f, expected > 80", north)
	}
	if math.Abs(south) > 0.001 {
		t.Errorf("south = %f, expected ~0", south)
	}
}

func TestTileBBoxInvalid(t *testing.T) {
	tests := []struct {
		z, x, y int
	}{
		{-1, 0, 0},
		{23, 0, 0},
		{1, 2, 0},  // x >= 2^z
		{1, 0, -1},
	}
	for _, tt := range tests {
		_, _, _, _, err := TileBBox(tt.z, tt.x, tt.y)
		if err == nil {
			t.Errorf("TileBBox(%d,%d,%d) expected error", tt.z, tt.x, tt.y)
		}
	}
}

func TestSignificanceThreshold(t *testing.T) {
	// At zoom 0: tile covers 360 degrees, so threshold = 360^2 / 256^2
	z0 := SignificanceThreshold(0)
	expected := (360.0 * 360.0) / (256.0 * 256.0)
	if math.Abs(z0-expected) > 1e-10 {
		t.Errorf("z=0: got %f, want %f", z0, expected)
	}

	// Higher zoom = lower threshold (more detail)
	z10 := SignificanceThreshold(10)
	if z10 >= z0 {
		t.Errorf("z=10 threshold %f should be less than z=0 threshold %f", z10, z0)
	}

	// Threshold should decrease by factor of 4 per zoom level
	z1 := SignificanceThreshold(1)
	ratio := z0 / z1
	if math.Abs(ratio-4) > 0.001 {
		t.Errorf("ratio z0/z1 = %f, want 4", ratio)
	}
}
