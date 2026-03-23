package node

import (
	"math"

	pb "monks.co/apps/breadcrumbs/proto"
)

// SimplifyMethod determines how significance is computed.
type SimplifyMethod string

const (
	// MethodArea uses pure Visvalingam-Whyatt triangle area.
	MethodArea SimplifyMethod = "area"
	// MethodDistance uses squared distance between neighbors — pure coverage,
	// no deviation awareness.
	MethodDistance SimplifyMethod = "distance"
	// MethodDistanceFloor uses max(triangleArea, distanceSquared) so that
	// collinear points along long segments still get high significance
	// while sharp turns are also preserved.
	MethodDistanceFloor SimplifyMethod = "distance_floor"
	// MethodMultiscale uses max(triangleArea, multiscale distance²). For each
	// point, it checks distance² at exponentially increasing offsets (1, 2, 4,
	// 8, ...) and takes the max. This creates a natural LOD pyramid: points at
	// "round" positions represent larger spans and survive at lower zoom.
	// Only usable via recompute (requires all points in memory).
	MethodMultiscale SimplifyMethod = "multiscale"
)

// ValidSimplifyMethod returns true if m is a known method.
func ValidSimplifyMethod(m string) bool {
	switch SimplifyMethod(m) {
	case MethodArea, MethodDistance, MethodDistanceFloor, MethodMultiscale:
		return true
	}
	return false
}

// Simplifier implements Visvalingam-Whyatt online simplification.
// It tracks the last two points and computes significance (triangle area
// in square degrees) for each appended point's predecessor.
type Simplifier struct {
	prev   *pb.Point // second-to-last
	tail   *pb.Point // last (its significance is always +Inf until a new point arrives)
	Method SimplifyMethod
}

// SimplifyResult holds the significance assignments from an Append call.
type SimplifyResult struct {
	// NewPointSig is the significance assigned to the newly appended point
	// (always +Inf, since it becomes the new tail).
	NewPointSig float64

	// PrevTailTimestamp is the timestamp of the point whose significance was recomputed.
	PrevTailTimestamp int64
	// PrevTailSig is the recomputed significance of the previous tail.
	PrevTailSig float64
	// HasPrevUpdate is true if a previous tail significance was recomputed.
	HasPrevUpdate bool
}

// NewSimplifier creates a simplifier with no state using the given method.
func NewSimplifier(method SimplifyMethod) *Simplifier {
	return &Simplifier{Method: method}
}

// Recover initializes the simplifier from the last two points in the store.
// Called on startup. prev may be nil if fewer than two points exist.
func (s *Simplifier) Recover(prev, tail *pb.Point) {
	s.prev = prev
	s.tail = tail
}

// Append processes a new point and returns significance assignments.
func (s *Simplifier) Append(p *pb.Point) SimplifyResult {
	if s.tail == nil {
		// First point ever.
		s.tail = p
		return SimplifyResult{NewPointSig: math.MaxFloat64}
	}

	if s.prev == nil {
		// Second point.
		s.prev = s.tail
		s.tail = p
		return SimplifyResult{NewPointSig: math.MaxFloat64}
	}

	// Third+ point: compute significance for (prev, tail, new).
	sig := computeSignificance(s.Method, s.prev, s.tail, p)
	result := SimplifyResult{
		NewPointSig:       math.MaxFloat64,
		PrevTailTimestamp: s.tail.Timestamp,
		PrevTailSig:       sig,
		HasPrevUpdate:     true,
	}

	s.prev = s.tail
	s.tail = p
	return result
}

// computeSignificance returns the significance for the middle point b
// given neighbors a and c, using the specified method.
func computeSignificance(method SimplifyMethod, a, b, c *pb.Point) float64 {
	switch method {
	case MethodDistance:
		return distanceSquared(a, c)
	case MethodDistanceFloor:
		return math.Max(triangleArea(a, b, c), distanceSquared(a, c))
	default:
		return triangleArea(a, b, c)
	}
}

// triangleArea computes the area of the triangle formed by three points
// in the lat/lon plane, in square degrees. Uses the shoelace formula.
func triangleArea(a, b, c *pb.Point) float64 {
	return math.Abs(
		(b.Longitude-a.Longitude)*(c.Latitude-a.Latitude)-
			(c.Longitude-a.Longitude)*(b.Latitude-a.Latitude),
	) / 2.0
}

// computeMultiscaleSignificance computes significance for point i given all
// points. It takes the max of triangleArea (immediate neighbors) and
// distanceSquared at exponentially increasing offsets (1, 2, 4, 8, ...).
func computeMultiscaleSignificance(pts []struct{ lat, lon float64 }, i int) float64 {
	n := len(pts)
	// Triangle area from immediate neighbors.
	a := &pb.Point{Latitude: pts[i-1].lat, Longitude: pts[i-1].lon}
	b := &pb.Point{Latitude: pts[i].lat, Longitude: pts[i].lon}
	c := &pb.Point{Latitude: pts[i+1].lat, Longitude: pts[i+1].lon}
	sig := triangleArea(a, b, c)

	// Distance² at doubling offsets.
	for k := 1; i-k >= 0 && i+k < n; k *= 2 {
		dlat := pts[i+k].lat - pts[i-k].lat
		dlon := pts[i+k].lon - pts[i-k].lon
		d2 := dlat*dlat + dlon*dlon
		if d2 > sig {
			sig = d2
		}
	}
	return sig
}

// distanceSquared returns the squared Euclidean distance between two points
// in the lat/lon plane, in square degrees.
func distanceSquared(a, c *pb.Point) float64 {
	dlat := c.Latitude - a.Latitude
	dlon := c.Longitude - a.Longitude
	return dlat*dlat + dlon*dlon
}
