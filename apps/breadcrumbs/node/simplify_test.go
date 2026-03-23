package node

import (
	"math"
	"testing"

	pb "monks.co/apps/breadcrumbs/proto"
)

func TestSimplifyFirstPoint(t *testing.T) {
	s := NewSimplifier(MethodArea)
	r := s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0})
	if r.NewPointSig != math.MaxFloat64 {
		t.Errorf("first point sig = %v, want MaxFloat64", r.NewPointSig)
	}
	if r.HasPrevUpdate {
		t.Error("first point should not have prev update")
	}
}

func TestSimplifySecondPoint(t *testing.T) {
	s := NewSimplifier(MethodArea)
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0})
	r := s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 1})
	if r.NewPointSig != math.MaxFloat64 {
		t.Errorf("second point sig = %v, want MaxFloat64", r.NewPointSig)
	}
	if r.HasPrevUpdate {
		t.Error("second point should not have prev update")
	}
}

func TestSimplifyThirdPoint(t *testing.T) {
	s := NewSimplifier(MethodArea)
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0})
	s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 0})
	r := s.Append(&pb.Point{Timestamp: 3, Latitude: 0, Longitude: 1})

	if r.NewPointSig != math.MaxFloat64 {
		t.Errorf("new point sig = %v, want MaxFloat64", r.NewPointSig)
	}
	if !r.HasPrevUpdate {
		t.Fatal("expected prev update")
	}
	if r.PrevTailTimestamp != 2 {
		t.Errorf("PrevTailTimestamp = %d, want 2", r.PrevTailTimestamp)
	}
	// Triangle: (0,0), (1,0), (0,1) has area 0.5
	if math.Abs(r.PrevTailSig-0.5) > 1e-10 {
		t.Errorf("PrevTailSig = %v, want 0.5", r.PrevTailSig)
	}
}

func TestSimplifyCollinear(t *testing.T) {
	s := NewSimplifier(MethodArea)
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0})
	s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 1})
	r := s.Append(&pb.Point{Timestamp: 3, Latitude: 2, Longitude: 2})

	if !r.HasPrevUpdate {
		t.Fatal("expected prev update")
	}
	if r.PrevTailSig != 0 {
		t.Errorf("collinear PrevTailSig = %v, want 0", r.PrevTailSig)
	}
}

func TestSimplifyDistancePure(t *testing.T) {
	s := NewSimplifier(MethodArea)
	s.Method = MethodDistance
	// (0,0), (1,0), (0,1): triangle area = 0.5, but pure distance only
	// uses distanceSquared((0,0), (0,1)) = 1.
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0})
	s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 0})
	r := s.Append(&pb.Point{Timestamp: 3, Latitude: 0, Longitude: 1})

	if !r.HasPrevUpdate {
		t.Fatal("expected prev update")
	}
	if r.PrevTailSig != 1.0 {
		t.Errorf("distance sig = %v, want 1.0 (dist² of (0,0)→(0,1))", r.PrevTailSig)
	}
}

func TestSimplifyDistanceFloorCollinear(t *testing.T) {
	s := NewSimplifier(MethodArea)
	s.Method = MethodDistanceFloor
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0})
	s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 1})
	r := s.Append(&pb.Point{Timestamp: 3, Latitude: 2, Longitude: 2})

	if !r.HasPrevUpdate {
		t.Fatal("expected prev update")
	}
	// Triangle area is 0 (collinear), but distance from (0,0) to (2,2) = sqrt(8),
	// so distanceSquared = 8. The floor should kick in.
	if r.PrevTailSig != 8.0 {
		t.Errorf("distance_floor collinear sig = %v, want 8.0", r.PrevTailSig)
	}
}

func TestSimplifyDistanceFloorUsesMaxOfAreaAndDistance(t *testing.T) {
	s := NewSimplifier(MethodArea)
	s.Method = MethodDistanceFloor
	// (0,0), (1,0), (0,1): triangle area = 0.5, distance (0,0)→(0,1) = 1, dist² = 1.
	// max(0.5, 1) = 1.
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0})
	s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 0})
	r := s.Append(&pb.Point{Timestamp: 3, Latitude: 0, Longitude: 1})

	if !r.HasPrevUpdate {
		t.Fatal("expected prev update")
	}
	if r.PrevTailSig != 1.0 {
		t.Errorf("distance_floor sig = %v, want 1.0 (max of area=0.5, dist²=1)", r.PrevTailSig)
	}
}

func TestAccuracyWeightGoodAccuracy(t *testing.T) {
	// Points with good accuracy (<= 10m) should have weight 1.0,
	// so significance is unchanged.
	s := NewSimplifier(MethodArea)
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0, HorizontalAccuracy: 5})
	s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 0, HorizontalAccuracy: 5})
	r := s.Append(&pb.Point{Timestamp: 3, Latitude: 0, Longitude: 1, HorizontalAccuracy: 5})

	if !r.HasPrevUpdate {
		t.Fatal("expected prev update")
	}
	// Triangle: (0,0), (1,0), (0,1) has area 0.5. All points have good accuracy.
	if math.Abs(r.PrevTailSig-0.5) > 1e-10 {
		t.Errorf("good accuracy sig = %v, want 0.5", r.PrevTailSig)
	}
}

func TestAccuracyWeightBadMiddlePoint(t *testing.T) {
	// A low-accuracy middle point should have its significance reduced.
	s := NewSimplifier(MethodArea)
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0, HorizontalAccuracy: 5})
	s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 0, HorizontalAccuracy: 1414})
	r := s.Append(&pb.Point{Timestamp: 3, Latitude: 0, Longitude: 1, HorizontalAccuracy: 5})

	if !r.HasPrevUpdate {
		t.Fatal("expected prev update")
	}
	// 1414m accuracy is above the 100m ceiling, so weight = 0, sig = 0.
	if r.PrevTailSig != 0 {
		t.Errorf("bad middle point sig = %v, want 0", r.PrevTailSig)
	}
}

func TestAccuracyWeightBadNeighbor(t *testing.T) {
	// A good point whose NEIGHBOR has bad accuracy should also get reduced
	// significance, because the triangle is untrustworthy.
	s := NewSimplifier(MethodArea)
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0, HorizontalAccuracy: 1414})
	s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 0, HorizontalAccuracy: 5})
	r := s.Append(&pb.Point{Timestamp: 3, Latitude: 0, Longitude: 1, HorizontalAccuracy: 5})

	if !r.HasPrevUpdate {
		t.Fatal("expected prev update")
	}
	// The triangle has one bad vertex (a) at 1414m (above 100m ceiling),
	// so weight = 0 and sig = 0.
	if r.PrevTailSig != 0 {
		t.Errorf("neighbor-of-bad sig = %v, want 0", r.PrevTailSig)
	}
}

func TestAccuracyWeightModerateAccuracy(t *testing.T) {
	// A point with 50m accuracy (between 10m baseline and 100m ceiling)
	// should have reduced but nonzero significance.
	s := NewSimplifier(MethodArea)
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0, HorizontalAccuracy: 5})
	s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 0, HorizontalAccuracy: 50})
	r := s.Append(&pb.Point{Timestamp: 3, Latitude: 0, Longitude: 1, HorizontalAccuracy: 5})

	if !r.HasPrevUpdate {
		t.Fatal("expected prev update")
	}
	// Raw area = 0.5, weight = (10/50)^2 = 0.04, so sig = 0.02
	want := 0.5 * (10.0 * 10.0) / (50.0 * 50.0)
	if math.Abs(r.PrevTailSig-want) > 1e-10 {
		t.Errorf("moderate accuracy sig = %v, want %v", r.PrevTailSig, want)
	}
}

func TestAccuracyWeightZeroAccuracy(t *testing.T) {
	// Zero accuracy (unknown) should be treated as good — CLLocation can report 0
	// before a fix is established, but protobuf default is 0 for older points.
	s := NewSimplifier(MethodArea)
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0, HorizontalAccuracy: 0})
	s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 0, HorizontalAccuracy: 0})
	r := s.Append(&pb.Point{Timestamp: 3, Latitude: 0, Longitude: 1, HorizontalAccuracy: 0})

	if !r.HasPrevUpdate {
		t.Fatal("expected prev update")
	}
	// With zero/unknown accuracy, weight should be 1.0 — no penalty.
	if math.Abs(r.PrevTailSig-0.5) > 1e-10 {
		t.Errorf("zero accuracy sig = %v, want 0.5", r.PrevTailSig)
	}
}

func TestAccuracyWeightDistanceFloor(t *testing.T) {
	// Accuracy weighting should also apply to distance_floor method.
	s := NewSimplifier(MethodDistanceFloor)
	s.Append(&pb.Point{Timestamp: 1, Latitude: 0, Longitude: 0, HorizontalAccuracy: 5})
	s.Append(&pb.Point{Timestamp: 2, Latitude: 1, Longitude: 1, HorizontalAccuracy: 1414})
	r := s.Append(&pb.Point{Timestamp: 3, Latitude: 2, Longitude: 2, HorizontalAccuracy: 5})

	if !r.HasPrevUpdate {
		t.Fatal("expected prev update")
	}
	// 1414m accuracy is above the 100m ceiling, so weight = 0, sig = 0.
	if r.PrevTailSig != 0 {
		t.Errorf("distance_floor bad accuracy sig = %v, want 0", r.PrevTailSig)
	}
}

func TestSimplifyRecovery(t *testing.T) {
	// Continuous: append 5 points, record the 5th's significance update.
	s1 := NewSimplifier(MethodArea)
	pts := []*pb.Point{
		{Timestamp: 1, Latitude: 0, Longitude: 0},
		{Timestamp: 2, Latitude: 1, Longitude: 0},
		{Timestamp: 3, Latitude: 1, Longitude: 1},
		{Timestamp: 4, Latitude: 2, Longitude: 1},
		{Timestamp: 5, Latitude: 2, Longitude: 2},
	}
	for _, p := range pts {
		s1.Append(p)
	}
	// Now append a 6th point with the continuous simplifier.
	r1 := s1.Append(&pb.Point{Timestamp: 6, Latitude: 3, Longitude: 2})

	// Recovered: start from last two points (4th and 5th), append 6th.
	s2 := NewSimplifier(MethodArea)
	s2.Recover(pts[3], pts[4])
	r2 := s2.Append(&pb.Point{Timestamp: 6, Latitude: 3, Longitude: 2})

	if !r1.HasPrevUpdate || !r2.HasPrevUpdate {
		t.Fatal("expected prev updates from both")
	}
	if r1.PrevTailSig != r2.PrevTailSig {
		t.Errorf("continuous sig %v != recovered sig %v", r1.PrevTailSig, r2.PrevTailSig)
	}
	if r1.PrevTailTimestamp != r2.PrevTailTimestamp {
		t.Errorf("continuous ts %d != recovered ts %d", r1.PrevTailTimestamp, r2.PrevTailTimestamp)
	}
}
