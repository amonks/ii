package creamery

import "fmt"

// FormulationTarget captures macro and functional targets inferred from a label
// or other specification source.
type FormulationTarget struct {
	Composition Composition

	Components ConstituentComponents
	Water      Interval
	POD        Interval
	PAC        Interval
}

// CompositionTarget returns the legacy composition portion of the goal.
func (t FormulationTarget) CompositionTarget() Composition {
	return t.canonicalComposition()
}

// HasPOD reports whether a usable POD range is present.
func (t FormulationTarget) HasPOD() bool {
	return t.POD.Hi > 0
}

// HasPAC reports whether a usable PAC range is present.
func (t FormulationTarget) HasPAC() bool {
	return t.PAC.Hi > 0
}

// String returns the composition representation for convenience.
func (t FormulationTarget) String() string {
	return t.canonicalComposition().String()
}

// ProteinInterval exposes the target protein interval.
func (t FormulationTarget) ProteinInterval() Interval {
	return t.Components.Protein
}

// LactoseInterval exposes the target lactose interval.
func (t FormulationTarget) LactoseInterval() Interval {
	return t.Components.Lactose
}

// AddedSugarsInterval exposes the summed interval for added sugars.
func (t FormulationTarget) AddedSugarsInterval() Interval {
	return t.Components.AddedSugarsInterval()
}

// TotalSugarsInterval exposes lactose plus added sugars.
func (t FormulationTarget) TotalSugarsInterval() Interval {
	return t.AddedSugarsInterval().Add(t.Components.Lactose)
}

// WaterInterval returns the explicit water interval constraint.
func (t FormulationTarget) WaterInterval() Interval {
	if intervalHasValue(t.Water) {
		return t.Water
	}
	if intervalHasValue(t.Components.Water) {
		return t.Components.Water
	}
	return Interval{}
}

// Validate ensures the target intervals are well-formed.
func (t FormulationTarget) Validate() error {
	if err := t.canonicalComposition().Valid(); err != nil {
		return err
	}
	if err := t.Components.Validate(); err != nil {
		return fmt.Errorf("invalid constituent targets: %w", err)
	}
	if t.Water.Lo < 0 || t.Water.Hi > 1 {
		return fmt.Errorf("target water interval out of range: %s", t.Water.String())
	}
	if t.Water.Lo > t.Water.Hi {
		return fmt.Errorf("target water interval has lo > hi: %s", t.Water.String())
	}
	return nil
}

// FormulationFromComposition creates a formulation target when only the legacy
// Composition is known (other fields default to zero intervals).
func FormulationFromComposition(comp Composition) FormulationTarget {
	components := comp.ToComponents()
	return FormulationTarget{
		Composition: comp,
		Components:  components,
		Water:       components.Water,
	}
}

func intervalHasValue(iv Interval) bool {
	return iv.Lo != 0 || iv.Hi != 0
}

func (t FormulationTarget) canonicalComposition() Composition {
	comp := t.Composition
	comps := t.Components

	if intervalHasValue(comps.Fat) {
		comp.Fat = comps.Fat
	}

	if intervalHasValue(comps.MSNF) || intervalHasValue(comps.Protein) || intervalHasValue(comps.Lactose) || intervalHasValue(comps.Ash) {
		comp.MSNF = comps.EffectiveMSNF()
	}

	if intervalHasValue(comps.Sucrose) ||
		intervalHasValue(comps.Glucose) ||
		intervalHasValue(comps.Fructose) ||
		intervalHasValue(comps.Maltodextrin) ||
		intervalHasValue(comps.Polyols) {
		comp.Sugar = comps.Sucrose.
			Add(comps.Glucose).
			Add(comps.Fructose).
			Add(comps.Maltodextrin).
			Add(comps.Polyols)
	}

	if intervalHasValue(comps.OtherSolids) {
		comp.Other = comps.OtherSolids
	}

	return comp
}
