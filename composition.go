package creamery

import "fmt"

// Composition represents the nutritional breakdown of an ingredient or target.
// All values are fractions (0-1), not percentages.
// Water is derived: 1 - (Fat + MSNF + Sugar + Other).
type Composition struct {
	Fat   Interval // milkfat
	MSNF  Interval // milk solids non-fat (protein, lactose, minerals)
	Sugar Interval // sweeteners
	Other Interval // stabilizers, cocoa solids, egg solids, etc.
}

// Water returns the water content, derived from other components.
func (c Composition) Water() Interval {
	// Water = 1 - (Fat + MSNF + Sugar + Other)
	// For intervals: if components are [lo, hi], water is [1-sumHi, 1-sumLo]
	sumLo := c.Fat.Lo + c.MSNF.Lo + c.Sugar.Lo + c.Other.Lo
	sumHi := c.Fat.Hi + c.MSNF.Hi + c.Sugar.Hi + c.Other.Hi
	return Interval{
		Lo: 1 - sumHi,
		Hi: 1 - sumLo,
	}
}

// TotalSolids returns the total solids (everything except water).
func (c Composition) TotalSolids() Interval {
	return Interval{
		Lo: c.Fat.Lo + c.MSNF.Lo + c.Sugar.Lo + c.Other.Lo,
		Hi: c.Fat.Hi + c.MSNF.Hi + c.Sugar.Hi + c.Other.Hi,
	}
}

// Valid checks if the composition is physically possible.
func (c Composition) Valid() error {
	if c.Fat.Lo < 0 || c.Fat.Hi > 1 {
		return fmt.Errorf("fat out of range: %v", c.Fat)
	}
	if c.MSNF.Lo < 0 || c.MSNF.Hi > 1 {
		return fmt.Errorf("MSNF out of range: %v", c.MSNF)
	}
	if c.Sugar.Lo < 0 || c.Sugar.Hi > 1 {
		return fmt.Errorf("sugar out of range: %v", c.Sugar)
	}
	if c.Other.Lo < 0 || c.Other.Hi > 1 {
		return fmt.Errorf("other out of range: %v", c.Other)
	}
	water := c.Water()
	if water.Lo < 0 {
		return fmt.Errorf("composition sums to more than 100%%: water would be %v", water)
	}
	return nil
}

// String returns a human-readable representation.
func (c Composition) String() string {
	return fmt.Sprintf("Fat: %s, MSNF: %s, Sugar: %s, Other: %s, Water: %s",
		c.Fat, c.MSNF, c.Sugar, c.Other, c.Water())
}

// PointComposition creates a composition with exact values.
func PointComposition(fat, msnf, sugar, other float64) Composition {
	return Composition{
		Fat:   Point(fat),
		MSNF:  Point(msnf),
		Sugar: Point(sugar),
		Other: Point(other),
	}
}

// Components returns all component intervals as a slice for iteration.
// Order: Fat, MSNF, Sugar, Other (water is derived).
func (c Composition) Components() []Interval {
	return []Interval{c.Fat, c.MSNF, c.Sugar, c.Other}
}

// ComponentNames returns the names corresponding to Components().
func ComponentNames() []string {
	return []string{"Fat", "MSNF", "Sugar", "Other"}
}

// CompositionFromProfile aggregates a constituent profile back into the legacy
// four-component composition (fat, MSNF, sugar, other).
func CompositionFromProfile(profile ConstituentProfile) Composition {
	msnf := profile.Components.Protein.Add(profile.Components.Lactose).Add(profile.Components.Ash)
	sugar := profile.Components.Sucrose.
		Add(profile.Components.Glucose).
		Add(profile.Components.Fructose).
		Add(profile.Components.Maltodextrin).
		Add(profile.Components.Polyols)
	return Composition{
		Fat:   profile.Components.Fat,
		MSNF:  msnf,
		Sugar: sugar,
		Other: profile.Components.OtherSolids,
	}
}
