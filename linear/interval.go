package linear

import (
	"fmt"
	"math"
)

// Interval represents a closed interval [Lo, Hi].
// For point values, Lo == Hi.
type Interval struct {
	Lo, Hi float64
}

// Point creates an interval representing a single value.
func Point(v float64) Interval {
	return Interval{Lo: v, Hi: v}
}

// Range creates an interval from lo to hi.
func Range(lo, hi float64) Interval {
	if lo > hi {
		lo, hi = hi, lo
	}
	return Interval{Lo: lo, Hi: hi}
}

// IsPoint returns true if this interval represents a single value.
func (i Interval) IsPoint() bool {
	return i.Lo == i.Hi
}

// Contains returns true if v is within the interval.
func (i Interval) Contains(v float64) bool {
	return v >= i.Lo && v <= i.Hi
}

// Overlaps returns true if two intervals share any points.
func (i Interval) Overlaps(other Interval) bool {
	return i.Lo <= other.Hi && other.Lo <= i.Hi
}

// Width returns Hi - Lo.
func (i Interval) Width() float64 {
	return i.Hi - i.Lo
}

// Mid returns the midpoint of the interval.
func (i Interval) Mid() float64 {
	return (i.Lo + i.Hi) / 2
}

// Add returns the interval sum: [a.Lo + b.Lo, a.Hi + b.Hi].
func (i Interval) Add(other Interval) Interval {
	return Interval{Lo: i.Lo + other.Lo, Hi: i.Hi + other.Hi}
}

// Sub returns the interval difference: [a.Lo - b.Hi, a.Hi - b.Lo].
func (i Interval) Sub(other Interval) Interval {
	return Interval{Lo: i.Lo - other.Hi, Hi: i.Hi - other.Lo}
}

// Mul returns the interval product.
func (i Interval) Mul(other Interval) Interval {
	products := []float64{
		i.Lo * other.Lo,
		i.Lo * other.Hi,
		i.Hi * other.Lo,
		i.Hi * other.Hi,
	}
	lo := products[0]
	hi := products[0]
	for _, p := range products[1:] {
		lo = math.Min(lo, p)
		hi = math.Max(hi, p)
	}
	return Interval{Lo: lo, Hi: hi}
}

// Scale multiplies the interval by a scalar.
func (i Interval) Scale(s float64) Interval {
	if s >= 0 {
		return Interval{Lo: i.Lo * s, Hi: i.Hi * s}
	}
	return Interval{Lo: i.Hi * s, Hi: i.Lo * s}
}

// Intersect returns the intersection of two intervals, or false if disjoint.
func (i Interval) Intersect(other Interval) (Interval, bool) {
	lo := math.Max(i.Lo, other.Lo)
	hi := math.Min(i.Hi, other.Hi)
	if lo > hi {
		return Interval{}, false
	}
	return Interval{Lo: lo, Hi: hi}, true
}

// Union returns the smallest interval containing both intervals.
func (i Interval) Union(other Interval) Interval {
	return Interval{
		Lo: math.Min(i.Lo, other.Lo),
		Hi: math.Max(i.Hi, other.Hi),
	}
}

// Clamp restricts a value to be within the interval.
func (i Interval) Clamp(v float64) float64 {
	if v < i.Lo {
		return i.Lo
	}
	if v > i.Hi {
		return i.Hi
	}
	return v
}

// String returns a human-readable representation.
func (i Interval) String() string {
	if i.IsPoint() {
		return fmt.Sprintf("%.2f%%", i.Lo*100)
	}
	return fmt.Sprintf("[%.2f%%, %.2f%%]", i.Lo*100, i.Hi*100)
}

// StringAbs returns a representation without percentage conversion.
func (i Interval) StringAbs() string {
	if i.IsPoint() {
		return fmt.Sprintf("%.2f", i.Lo)
	}
	return fmt.Sprintf("[%.2f, %.2f]", i.Lo, i.Hi)
}
