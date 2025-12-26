package creamery

import "fmt"

// FormulationTarget captures canonical constituent targets inferred from labels
// or other specification sources. All constraints are expressed as fractions
// (0-1) of total mix mass.
type FormulationTarget struct {
	Components CompositionRange
	POD        Interval
	PAC        Interval
}

// FormulationFromFractions builds a target using the provided component
// fractions, deriving the water interval when omitted.
func FormulationFromFractions(f CompositionRange) FormulationTarget {
	withMSNF := populateMSNFComponents(f)
	return FormulationTarget{Components: EnsureWater(withMSNF)}
}

// HasPOD reports whether a usable POD range is present.
func (t FormulationTarget) HasPOD() bool { return t.POD.Hi > 0 }

// HasPAC reports whether a usable PAC range is present.
func (t FormulationTarget) HasPAC() bool { return t.PAC.Hi > 0 }

// String returns a compact component summary.
func (t FormulationTarget) String() string {
	return ComponentSummary(t.Components)
}

// FatInterval exposes the fat target.
func (t FormulationTarget) FatInterval() Interval {
	return t.Components.Fat
}

// MSNFInterval exposes the MSNF target (derived when unspecified).
func (t FormulationTarget) MSNFInterval() Interval {
	return t.Components.EffectiveMSNF()
}

// ProteinInterval exposes the protein target.
func (t FormulationTarget) ProteinInterval() Interval {
	return t.Components.Protein
}

// LactoseInterval exposes the lactose target.
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

// OtherSolidsInterval exposes non-sugar other solids.
func (t FormulationTarget) OtherSolidsInterval() Interval {
	return t.Components.OtherSolids
}

// WaterInterval returns the explicit water interval constraint.
func (t FormulationTarget) WaterInterval() Interval {
	return t.Components.Water
}

// Validate ensures the target intervals are well-formed.
func (t FormulationTarget) Validate() error {
	if err := t.Components.Validate(); err != nil {
		return fmt.Errorf("invalid constituent targets: %w", err)
	}
	if t.Components.Water.Lo < 0 || t.Components.Water.Hi > 1 {
		return fmt.Errorf("target water interval out of range: %s", t.Components.Water.String())
	}
	if t.Components.Water.Lo > t.Components.Water.Hi {
		return fmt.Errorf("target water interval has lo > hi: %s", t.Components.Water.String())
	}
	return nil
}
