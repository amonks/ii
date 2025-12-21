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
	return c.ToComponents().Validate()
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

// ToComponents expands the four-part composition into constituent components
// using canonical dairy assumptions.
func (c Composition) ToComponents() ConstituentComponents {
	components := ConstituentComponents{
		Fat:         c.Fat,
		MSNF:        c.MSNF,
		Sucrose:     c.Sugar,
		OtherSolids: c.Other,
		Water:       c.Water(),
	}
	components.Protein = c.MSNF.Scale(proteinFractionOfMSNF)
	components.Lactose = c.MSNF.Scale(LactoseFractionOfMSNF)
	components.Ash = clampInterval(c.MSNF.Sub(components.Protein.Add(components.Lactose)), 0)
	return components
}

// CompositionFromProfile aggregates a constituent profile back into the legacy
// four-component composition (fat, MSNF, sugar, other).
func CompositionFromProfile(profile ConstituentProfile) Composition {
	return CompositionFromComponents(profile.Components)
}

// CompositionFromComponents aggregates constituent components back into the four-part composition.
func CompositionFromComponents(components ConstituentComponents) Composition {
	sugar := components.Sucrose.
		Add(components.Glucose).
		Add(components.Fructose).
		Add(components.Maltodextrin).
		Add(components.Polyols)
	return Composition{
		Fat:   components.Fat,
		MSNF:  components.EffectiveMSNF(),
		Sugar: sugar,
		Other: components.OtherSolids,
	}
}
