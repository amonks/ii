package linear

const (
	// Molecular weights (g/mol)
	mwSucrose    = 342.30
	mwGlucose    = 180.16
	mwFructose   = 180.16
	mwLactose    = 342.30
	mwGlycerol   = 92.09
	mwSorbitol   = 182.17
	mwErythritol = 122.12

	// Cryoscopic constant for water (°C·kg/mol)
	kfWater = 1.86

	// Volume & density helpers
	mixDensityKgPerL     = 1.02
	pintLiters           = 0.473
	usCupLiters          = 0.236588
	servingPortionLiters = usCupLiters * (2.0 / 3.0)

	// Default process assumptions
	defaultServeTempC = -12.0
	defaultDrawTempC  = -5.0
	defaultShearRate  = 50.0

	// Composition heuristics
	proteinFractionOfMSNF = 0.38
	mineralFractionOfMSNF = 0.08

	labelPercentEPS = 0.02
)

const (
	dairyTransFatShare = 0.035
	eggTransFatShare   = 0.01
)

func mgPerKgFrom100g(value float64) float64 {
	return value * 10.0
}

// DefaultServeTempC exposes the default serving temperature used in physics helpers.
func DefaultServeTempC() float64 { return defaultServeTempC }

// DefaultDrawTempC exposes the default draw temperature.
func DefaultDrawTempC() float64 { return defaultDrawTempC }

// DefaultShearRate exposes the default shear rate.
func DefaultShearRate() float64 { return defaultShearRate }

// ServingPortionLiters returns the default scoop volume used for nutrition calculations.
func ServingPortionLiters() float64 { return servingPortionLiters }
