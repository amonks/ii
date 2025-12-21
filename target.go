package creamery

// FormulationTarget captures macro and functional targets inferred from a label
// or other specification source.
type FormulationTarget struct {
	Composition Composition

	Protein     Interval
	Lactose     Interval
	AddedSugars Interval
	TotalSugars Interval
	Water       Interval
	POD         Interval
	PAC         Interval
}

// CompositionTarget returns the legacy composition portion of the goal.
func (t FormulationTarget) CompositionTarget() Composition {
	return t.Composition
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
	return t.Composition.String()
}

// FormulationFromComposition creates a formulation target when only the legacy
// Composition is known (other fields default to zero intervals).
func FormulationFromComposition(comp Composition) FormulationTarget {
	return FormulationTarget{
		Composition: comp,
	}
}
