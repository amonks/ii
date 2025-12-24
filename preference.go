package creamery

import (
	"math"
)

// RecipePreference aggregates multiple physical targets into a single score.
type RecipePreference struct {
	Viscosity   ViscosityPreference
	Sweetness   SweetnessPreference
	IceFraction IceFractionPreference
	PAC         PACPreference
	POD         PODPreference
}

// DefaultRecipePreference combines viscosity, sweetness, and ice fraction
// curves tuned for scoopable premium ice cream.
func DefaultRecipePreference() RecipePreference {
	return RecipePreference{
		Viscosity:   DefaultViscosityPreference(),
		Sweetness:   DefaultSweetnessPreference(),
		IceFraction: DefaultIceFractionPreference(),
		PAC:         DefaultPACPreference(),
		POD:         DefaultPODPreference(),
	}
}

// Score multiplies the component scores to keep the response smooth for NLopt.
func (rp RecipePreference) Score(snapshot BatchSnapshot) float64 {
	score := 1.0
	mass := math.Max(1e-9, snapshot.TotalMassKg)

	score *= rp.Viscosity.Score(snapshot.ViscosityAtServe)
	sweetnessPct := snapshot.SweetnessEq / mass
	score *= rp.Sweetness.Score(sweetnessPct)
	score *= rp.IceFraction.Score(snapshot.IceFractionAtServe)
	score *= rp.PAC.Score(snapshot.Sweeteners.TotalPAC)
	score *= rp.POD.Score(snapshot.Sweeteners.TotalPOD)

	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

// ViscosityPreference encodes a sigmoid window describing which viscosity
// values should be preferred without outright rejecting other options.
type ViscosityPreference struct {
	// Lower marks the start of the preferred viscosity window (Pa·s).
	Lower float64
	// Upper marks the end of the preferred viscosity window (Pa·s).
	Upper float64
	// Transition defines how quickly the preference falls off (Pa·s).
	Transition float64
	// Floor ensures every recipe retains some non-zero weight.
	Floor float64
}

// DefaultViscosityPreference returns a preference tuned for scoopable premium
// ice cream served around -12°C to -10°C. It favors ~3.5–4.1 mPa·s while still
// assigning a non-zero weight outside that window.
func DefaultViscosityPreference() ViscosityPreference {
	return ViscosityPreference{
		Lower:      0.0039,
		Upper:      0.0047,
		Transition: 0.00035,
		Floor:      0.05,
	}
}

// Score returns a normalized preference weight in [0, 1].
func (vp ViscosityPreference) Score(viscosity float64) float64 {
	if viscosity <= 0 {
		return clampFloat(vp.Floor, 0, 0.95)
	}
	return sigmoidScore(viscosity, vp.Lower, vp.Upper, vp.Transition, vp.Floor)
}

// SweetnessPreference targets sucrose-equivalent percentages.
type SweetnessPreference struct {
	Lower      float64
	Upper      float64
	Transition float64
	Floor      float64
}

// DefaultSweetnessPreference biases toward ~13-14% sucrose-equivalent solids,
// slightly below the 14.6% benchmark recorded in the workflow log.
func DefaultSweetnessPreference() SweetnessPreference {
	return SweetnessPreference{
		Lower:      0.13, // 13%
		Upper:      0.14, // 14%
		Transition: 0.01, // smooth drop outside window
		Floor:      0.15, // keep impact softer than viscosity
	}
}

func (sp SweetnessPreference) Score(sweetness float64) float64 {
	if sweetness <= 0 {
		return clampFloat(sp.Floor, 0, 0.99)
	}
	return sigmoidScore(sweetness, sp.Lower, sp.Upper, sp.Transition, sp.Floor)
}

// IceFractionPreference targets the amount of ice present at serving
// temperature. Higher fractions tend to yield firmer, drier ice cream.
type IceFractionPreference struct {
	Lower      float64
	Upper      float64
	Transition float64
	Floor      float64
}

// DefaultIceFractionPreference favors ~55-65% ice at serve temperature to
// maintain body without going icy.
func DefaultIceFractionPreference() IceFractionPreference {
	return IceFractionPreference{
		Lower:      0.55,
		Upper:      0.65,
		Transition: 0.05,
		Floor:      0.2,
	}
}

func (ip IceFractionPreference) Score(iceFraction float64) float64 {
	if iceFraction <= 0 {
		return clampFloat(ip.Floor, 0, 0.95)
	}
	return sigmoidScore(iceFraction, ip.Lower, ip.Upper, ip.Transition, ip.Floor)
}

// PACPreference biases toward desirable freezing point depression.
type PACPreference struct {
	Lower      float64
	Upper      float64
	Transition float64
	Floor      float64
}

// DefaultPACPreference favors 24-32 PAC, a band that stays scoopable without
// veering into soupy territory.
func DefaultPACPreference() PACPreference {
	return PACPreference{
		Lower:      24,
		Upper:      32,
		Transition: 3,
		Floor:      0.25,
	}
}

func (pp PACPreference) Score(pac float64) float64 {
	if pac <= 0 {
		return clampFloat(pp.Floor, 0, 0.95)
	}
	return sigmoidScore(pac, pp.Lower, pp.Upper, pp.Transition, pp.Floor)
}

// PODPreference tracks sucrose-equivalent intensity (TotalPOD).
type PODPreference struct {
	Lower      float64
	Upper      float64
	Transition float64
	Floor      float64
}

// DefaultPODPreference nudges recipes toward ~15-17 relative POD, matching
// premium vanilla targets pulled from label analyses.
func DefaultPODPreference() PODPreference {
	return PODPreference{
		Lower:      15,
		Upper:      17,
		Transition: 1,
		Floor:      0.3,
	}
}

func (pp PODPreference) Score(pod float64) float64 {
	if pod <= 0 {
		return clampFloat(pp.Floor, 0, 0.99)
	}
	return sigmoidScore(pod, pp.Lower, pp.Upper, pp.Transition, pp.Floor)
}

func sigmoidScore(value, lower, upper, transition, floor float64) float64 {
	if upper <= lower {
		upper = lower + math.Max(transition, 1e-4)
	}
	if transition <= 0 {
		transition = (upper - lower) / 3
		if transition <= 0 {
			transition = 1e-4
		}
	}
	slope := 1.0 / transition
	lowerSig := 1.0 / (1.0 + math.Exp(-slope*(value-lower)))
	upperSig := 1.0 / (1.0 + math.Exp(slope*(value-upper)))
	bump := lowerSig * upperSig

	floor = clampFloat(floor, 0, 0.99)
	score := floor + (1-floor)*bump
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

// WithPACWindow returns a copy of the preference with the PAC curve retargeted
// to the requested window.
func (rp RecipePreference) WithPACWindow(lower, upper float64) RecipePreference {
	pref := rp
	if pref.PAC == (PACPreference{}) {
		pref.PAC = DefaultPACPreference()
	}
	pref.PAC.Lower = lower
	pref.PAC.Upper = upper
	if pref.PAC.Transition <= 0 {
		pref.PAC.Transition = defaultPreferenceTransition(lower, upper)
	}
	if pref.PAC.Floor <= 0 {
		pref.PAC.Floor = DefaultPACPreference().Floor
	}
	return pref
}

// WithPODWindow returns a copy of the preference with the POD curve retargeted.
func (rp RecipePreference) WithPODWindow(lower, upper float64) RecipePreference {
	pref := rp
	if pref.POD == (PODPreference{}) {
		pref.POD = DefaultPODPreference()
	}
	pref.POD.Lower = lower
	pref.POD.Upper = upper
	if pref.POD.Transition <= 0 {
		pref.POD.Transition = defaultPreferenceTransition(lower, upper)
	}
	if pref.POD.Floor <= 0 {
		pref.POD.Floor = DefaultPODPreference().Floor
	}
	return pref
}

func defaultPreferenceTransition(lower, upper float64) float64 {
	width := upper - lower
	if width <= 0 {
		return 1
	}
	return math.Max(width/4, 1e-4)
}

func normalizeRecipePreference(pref RecipePreference) RecipePreference {
	if pref.Viscosity == (ViscosityPreference{}) {
		pref.Viscosity = DefaultViscosityPreference()
	}
	if pref.Sweetness == (SweetnessPreference{}) {
		pref.Sweetness = DefaultSweetnessPreference()
	}
	if pref.IceFraction == (IceFractionPreference{}) {
		pref.IceFraction = DefaultIceFractionPreference()
	}
	if pref.PAC == (PACPreference{}) {
		pref.PAC = DefaultPACPreference()
	}
	if pref.POD == (PODPreference{}) {
		pref.POD = DefaultPODPreference()
	}
	return pref
}
