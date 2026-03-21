package node

import (
	"math"

	pb "monks.co/apps/breadcrumbs/proto"
)

// Simplifier implements Visvalingam-Whyatt online simplification.
// It tracks the last two points and computes significance (triangle area
// in square degrees) for each appended point's predecessor.
type Simplifier struct {
	prev *pb.Point // second-to-last
	tail *pb.Point // last (its significance is always +Inf until a new point arrives)
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

// NewSimplifier creates a simplifier with no state.
func NewSimplifier() *Simplifier {
	return &Simplifier{}
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

	// Third+ point: compute triangle area for (prev, tail, new).
	area := triangleArea(s.prev, s.tail, p)
	result := SimplifyResult{
		NewPointSig:       math.MaxFloat64,
		PrevTailTimestamp: s.tail.Timestamp,
		PrevTailSig:       area,
		HasPrevUpdate:     true,
	}

	s.prev = s.tail
	s.tail = p
	return result
}

// triangleArea computes the area of the triangle formed by three points
// in the lat/lon plane, in square degrees. Uses the shoelace formula.
func triangleArea(a, b, c *pb.Point) float64 {
	return math.Abs(
		(b.Longitude-a.Longitude)*(c.Latitude-a.Latitude)-
			(c.Longitude-a.Longitude)*(b.Latitude-a.Latitude),
	) / 2.0
}
