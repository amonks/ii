package linear

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

	// Corn syrup (42 DE) - less sweet, moderate PAC
	CornSyrup42POD = 50
	CornSyrup42PAC = 90

	// Corn syrup solids (36 DE)
	CornSyrupSolids36POD = 35
	CornSyrupSolids36PAC = 80

	// Invert sugar
	InvertSugarPOD = 130
	InvertSugarPAC = 190

	// Maltodextrin (low DE) - barely sweet, low PAC
	MaltodextrinPOD = 10
	MaltodextrinPAC = 25

	// Trehalose - half as sweet, similar PAC
	TrehalosePOD = 45
	TrehalosePAC = 100

	// Tapioca syrup (similar to corn syrup ~42 DE)
	TapiocaSyrupPOD = 50
	TapiocaSyrupPAC = 90
)

// SweetenerProps holds POD and PAC for a sweetener.
type SweetenerProps struct {
	POD float64 // sweetening power (sucrose = 100)
	PAC float64 // anti-freezing power (sucrose = 100)
}

// Common sweetener properties
var (
	Sucrose       = SweetenerProps{POD: SucrosePOD, PAC: SucrosePAC}
	Dextrose      = SweetenerProps{POD: DextrosePOD, PAC: DextrosePAC}
	Fructose      = SweetenerProps{POD: FructosePOD, PAC: FructosePAC}
	Lactose       = SweetenerProps{POD: LactosePOD, PAC: LactosePAC}
	CornSyrup42   = SweetenerProps{POD: CornSyrup42POD, PAC: CornSyrup42PAC}
	InvertSugar   = SweetenerProps{POD: InvertSugarPOD, PAC: InvertSugarPAC}
	Maltodextrin  = SweetenerProps{POD: MaltodextrinPOD, PAC: MaltodextrinPAC}
	TapiocaSyrupS = SweetenerProps{POD: TapiocaSyrupPOD, PAC: TapiocaSyrupPAC}
)

// LactoseFractionOfMSNF is the approximate fraction of MSNF that is lactose.
// MSNF is roughly: 38% protein, 54% lactose, 8% minerals
const LactoseFractionOfMSNF = 0.54

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
	AddedSugarPOD  float64
	AddedSugarPAC  float64
	LactosePOD     float64
	LactosePAC     float64
}

// AnalyzeSweeteners computes POD/PAC for a solution.
func AnalyzeSweeteners(sol *Solution, ingredients []Ingredient) SweetenerAnalysis {
	var analysis SweetenerAnalysis

	for _, ing := range ingredients {
		w := sol.Weights[ing.Name]
		if w < 0.001 {
			continue
		}

		// Lactose contribution from MSNF
		msnf := ing.Comp.MSNF.Mid()
		lactose := msnf * LactoseFractionOfMSNF * w
		analysis.LactosePOD += lactose * LactosePOD
		analysis.LactosePAC += lactose * LactosePAC

		// Added sugar contribution
		sugar := ing.Comp.Sugar.Mid() * w
		if sugar > 0 && ing.Sweetener.POD > 0 {
			analysis.AddedSugarPOD += sugar * ing.Sweetener.POD
			analysis.AddedSugarPAC += sugar * ing.Sweetener.PAC
		}
	}

	analysis.TotalPOD = analysis.AddedSugarPOD + analysis.LactosePOD
	analysis.TotalPAC = analysis.AddedSugarPAC + analysis.LactosePAC

	return analysis
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
