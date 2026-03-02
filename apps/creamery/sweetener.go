package creamery

import "math"

// Sweetener properties for ice cream formulation.
//
// POD (Potere Dolcificante / Sweetening Power): relative sweetness vs sucrose = 100
// PAC (Potere Anti-Congelante / Anti-freezing Power): freezing point depression vs sucrose = 100
//
// These are linear approximations. The actual physics of freezing point depression
// is non-linear, but the linear model works well enough for formulation.

// Common sweetener POD/PAC values (relative to sucrose = 100)
const (
	// Sucrose (table sugar)
	SucrosePOD = 100
	SucrosePAC = 100

	// Dextrose (glucose) - less sweet, more freezing depression
	DextrosePOD = 75
	DextrosePAC = 180

	// Fructose - sweeter, more freezing depression
	FructosePOD = 170
	FructosePAC = 190

	// Lactose (milk sugar) - barely sweet, similar PAC to sucrose
	LactosePOD = 16
	LactosePAC = 100

	// Maltodextrin (low DE) - barely sweet, low PAC
	MaltodextrinPOD = 10

	// Polyols (typical sweetness ~0.6 vs sucrose)
	PolyolPOD = 60
)

// LactoseFractionOfMSNF is the approximate fraction of MSNF that is lactose.
// MSNF is roughly: 38% protein, 54% lactose, 8% minerals
const LactoseFractionOfMSNF = 0.54

const (
	defaultMaltodextrinDP = 10.0
	defaultPolyolMW       = mwSorbitol
	pacPerMole            = mwSucrose / 10.0
)

func osmoticFactor(f ConstituentFunctionals) float64 {
	coeff := f.OsmoticCoeff
	if coeff == 0 {
		coeff = 1
	}
	vh := f.VHFactor
	if vh == 0 {
		vh = 1
	}
	return coeff * vh
}

func maltodextrinDP(f ConstituentFunctionals) float64 {
	if f.MaltodextrinDP > 0 {
		return f.MaltodextrinDP
	}
	return defaultMaltodextrinDP
}

func polyolMW(f ConstituentFunctionals) float64 {
	if f.PolyolMW > 0 {
		return f.PolyolMW
	}
	return defaultPolyolMW
}

type sugarMasses struct {
	sucrose      float64
	glucose      float64
	fructose     float64
	maltodextrin float64
	polyols      float64
}

func addedPODFromMasses(m sugarMasses) float64 {
	return m.sucrose*SucrosePOD +
		m.glucose*DextrosePOD +
		m.fructose*FructosePOD +
		m.maltodextrin*MaltodextrinPOD +
		m.polyols*PolyolPOD
}

func addedPACFromMasses(m sugarMasses, funcs ConstituentFunctionals) float64 {
	dp := maltodextrinDP(funcs)
	polyMW := polyolMW(funcs)
	factor := osmoticFactor(funcs)
	moles := m.sucrose*1000.0/mwSucrose +
		m.glucose*1000.0/mwGlucose +
		m.fructose*1000.0/mwFructose +
		m.maltodextrin*1000.0/(mwGlucose*dp) +
		m.polyols*1000.0/polyMW
	return moles * pacPerMole * factor
}

func lactosePACFromMass(lactose float64, funcs ConstituentFunctionals) float64 {
	moles := lactose * 1000.0 / mwLactose
	return moles * pacPerMole * osmoticFactor(funcs)
}

func profileAddedPOD(profile ConstituentProfile) Interval {
	comps := profile.Components
	pod := comps.Sucrose.Scale(SucrosePOD).
		Add(comps.Glucose.Scale(DextrosePOD)).
		Add(comps.Fructose.Scale(FructosePOD)).
		Add(comps.Maltodextrin.Scale(MaltodextrinPOD)).
		Add(comps.Polyols.Scale(PolyolPOD))
	return pod
}

func pacIntervalsFromProfile(profile ConstituentProfile) (Interval, Interval) {
	comps := profile.Components
	funcs := profile.Functionals

	factor := osmoticFactor(funcs)
	dp := maltodextrinDP(funcs)
	polyMW := polyolMW(funcs)

	addedMoles := Interval{}
	addedMoles = addedMoles.Add(comps.Sucrose.Scale(1000.0 / mwSucrose))
	addedMoles = addedMoles.Add(comps.Glucose.Scale(1000.0 / mwGlucose))
	addedMoles = addedMoles.Add(comps.Fructose.Scale(1000.0 / mwFructose))
	addedMoles = addedMoles.Add(comps.Maltodextrin.Scale(1000.0 / (mwGlucose * dp)))
	addedMoles = addedMoles.Add(comps.Polyols.Scale(1000.0 / polyMW))
	if funcs.EffectiveMW > 0 {
		addedMoles = addedMoles.Add(comps.OtherSolids.Scale(1000.0 / funcs.EffectiveMW))
	}
	addedMoles = addedMoles.Scale(factor)

	lactoseMoles := comps.Lactose.Scale(1000.0 / mwLactose).Scale(factor)

	return addedMoles.Scale(pacPerMole), lactoseMoles.Scale(pacPerMole)
}

// AddedPODInterval returns the added sugar sweetness contribution.
func (p ConstituentProfile) AddedPODInterval() Interval {
	return profileAddedPOD(p)
}

// LactosePODInterval returns the lactose sweetness contribution.
func (p ConstituentProfile) LactosePODInterval() Interval {
	return p.Components.Lactose.Scale(LactosePOD)
}

// PODInterval returns the total sweetness contribution.
func (p ConstituentProfile) PODInterval() Interval {
	return p.AddedPODInterval().Add(p.LactosePODInterval())
}

// AddedPACInterval returns the freezing point depression from added sugars.
func (p ConstituentProfile) AddedPACInterval() Interval {
	added, _ := pacIntervalsFromProfile(p)
	return added
}

// LactosePACInterval returns the freezing point depression from lactose.
func (p ConstituentProfile) LactosePACInterval() Interval {
	_, lactose := pacIntervalsFromProfile(p)
	return lactose
}

// PACInterval returns the total freezing point depression contribution.
func (p ConstituentProfile) PACInterval() Interval {
	added, lactose := pacIntervalsFromProfile(p)
	return added.Add(lactose)
}

func addSweetenerContribution(analysis *SweetenerAnalysis, profile ConstituentProfile, weight float64) {
	if analysis == nil || weight <= 0 {
		return
	}
	analysis.AddedSugarPOD += weight * profile.AddedPODInterval().Mid()
	analysis.LactosePOD += weight * profile.LactosePODInterval().Mid()
	analysis.AddedSugarPAC += weight * profile.AddedPACInterval().Mid()
	analysis.LactosePAC += weight * profile.LactosePACInterval().Mid()
}

func finalizeSweetenerTotals(analysis *SweetenerAnalysis) {
	if analysis == nil {
		return
	}
	analysis.TotalPOD = analysis.AddedSugarPOD + analysis.LactosePOD
	analysis.TotalPAC = analysis.AddedSugarPAC + analysis.LactosePAC
}

func normalizeSweetenerTotals(analysis *SweetenerAnalysis, totalMass float64) {
	if analysis == nil {
		return
	}
	if totalMass <= 0 {
		return
	}
	inv := 1 / totalMass
	analysis.TotalPOD *= inv
	analysis.TotalPAC *= inv
	analysis.AddedSugarPOD *= inv
	analysis.AddedSugarPAC *= inv
	analysis.LactosePOD *= inv
	analysis.LactosePAC *= inv
}

// SweetenerAnalysis calculates POD and PAC for a solution.
// This accounts for:
// - Added sugars (using each ingredient's sweetener properties)
// - Lactose from MSNF (dairy ingredients)
type SweetenerAnalysis struct {
	// Total sweetness relative to equivalent sucrose
	TotalPOD float64

	// Total freezing point depression relative to equivalent sucrose
	TotalPAC float64

	// Breakdown
	AddedSugarPOD float64
	AddedSugarPAC float64
	LactosePOD    float64
	LactosePAC    float64
}

// AnalyzeSweeteners computes POD/PAC for a solution.
func AnalyzeSweeteners(sol *Solution, specs []Ingredient) SweetenerAnalysis {
	if sol == nil {
		return SweetenerAnalysis{}
	}
	if len(sol.Blend.Components) > 0 {
		return SweetenersFromBatch(NewBatch(sol.Blend, 1))
	}
	components, err := componentsFromSolution(sol, specs, 1)
	if err != nil {
		return SweetenerAnalysis{}
	}
	return SweetenersFromComponents(components)
}

// SweetenersFromRecipe aggregates POD/PAC directly from a recipe definition.
func SweetenersFromRecipe(recipe *Recipe) SweetenerAnalysis {
	if recipe == nil {
		return SweetenerAnalysis{}
	}
	return SweetenersFromBatch(recipe.batch())
}

// SweetenersFromComponents aggregates POD/PAC for arbitrary recipe components.
func SweetenersFromComponents(components []RecipeComponent) SweetenerAnalysis {
	var analysis SweetenerAnalysis
	totalMass := 0.0
	for _, comp := range components {
		if comp.MassKg <= 0 {
			continue
		}
		totalMass += comp.MassKg
		profile := comp.Ingredient.EffectiveProfile()
		addSweetenerContribution(&analysis, profile, comp.MassKg)
	}
	finalizeSweetenerTotals(&analysis)
	normalizeSweetenerTotals(&analysis, totalMass)
	return analysis
}

// SweetenersFromBatch aggregates POD/PAC directly from a batch abstraction.
func SweetenersFromBatch(batch Batch) SweetenerAnalysis {
	return SweetenersFromComponents(batch.Components())
}

// EquivalentSucrose returns the amount of sucrose that would give the same sweetness.
func (a SweetenerAnalysis) EquivalentSucrose() float64 {
	return a.TotalPOD / 100
}

// RelativeSoftness returns a qualitative measure of how soft the ice cream will be.
// Higher PAC = softer at serving temperature.
// Typical range: 20-35 for scoopable ice cream.
func (a SweetenerAnalysis) RelativeSoftness() string {
	pac := a.TotalPAC
	if math.IsNaN(pac) || math.IsInf(pac, 0) {
		return "softness unavailable"
	}
	switch {
	case pac < 20:
		return "very hard"
	case pac < 25:
		return "hard"
	case pac < 30:
		return "firm (good for scooping)"
	case pac < 35:
		return "soft (good for serving)"
	case pac < 40:
		return "very soft"
	default:
		return "too soft (may not hold shape)"
	}
}
