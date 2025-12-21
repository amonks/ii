package creamery

import (
	"errors"
	"math"
)

// MixOptions configures the physics calculations.
type MixOptions struct {
	ServeTempC   float64
	DrawTempC    float64
	ShearRate    float64
	OverrunCap   float64
	LimitOverrun bool
}

func defaultMixOptions(opts MixOptions) MixOptions {
	if opts.ServeTempC == 0 {
		opts.ServeTempC = defaultServeTempC
	}
	if opts.DrawTempC == 0 {
		opts.DrawTempC = defaultDrawTempC
	}
	if opts.ShearRate == 0 {
		opts.ShearRate = defaultShearRate
	}
	return opts
}

// IntervalMass tracks the accumulated mass contribution of an interval-valued component.
type IntervalMass struct {
	Lo  float64
	Mid float64
	Hi  float64
}

func (m *IntervalMass) add(iv Interval, mass float64) {
	m.Lo += mass * iv.Lo
	m.Hi += mass * iv.Hi
	m.Mid += mass * iv.Mid()
}

// BatchSnapshot captures aggregated component totals and derived process metrics for a recipe mix.
type BatchSnapshot struct {
	TotalMassKg float64

	WaterMassKg            float64
	FatMassKg              float64
	ProteinMassKg          float64
	Lactose                IntervalMass
	SucroseMassKg          float64
	GlucoseMassKg          float64
	FructoseMassKg         float64
	MaltodextrinMassKg     float64
	PolyolsMassKg          float64
	AshMassKg              float64
	OtherSolidsMassKg      float64
	TransFatMassKg         float64
	SaturatedFat           IntervalMass
	AddedSugars            IntervalMass
	EmulsifierPower        float64
	EmulsifierMassKg       float64
	BoundWaterKg           float64
	PolymerSolidsKg        float64
	ColligativeMoles       float64
	CholesterolMgTotal     float64
	SweetnessEq            float64
	CostTotal              float64
	MixVolumeL             float64
	WaterPct               float64
	SolidsPct              float64
	FatPct                 float64
	ProteinPct             float64
	LactosePct             float64
	LactoseMinPct          float64
	LactoseMaxPct          float64
	TotalSugarsPct         float64
	AddedSugarsPct         float64
	AddedSugarsMinPct      float64
	AddedSugarsMaxPct      float64
	TransFatPct            float64
	SaturatedFatPct        float64
	SaturatedFatMinPct     float64
	SaturatedFatMaxPct     float64
	CholesterolMgPerKg     float64
	PolymerSolidsPct       float64
	CostPerKg              float64
	PintsYield             float64
	CostPerPint            float64
	FreezingPointC         float64
	IceFractionAtServe     float64
	ViscosityAtServe       float64
	OverrunEstimate        float64
	HardnessIndex          float64
	MeltdownIndex          float64
	LactoseSupersaturation float64
	FreezerLoadKJ          float64
}

// NewBatchSnapshot aggregates ingredient components without applying process options.
func NewBatchSnapshot(components []RecipeComponent) (BatchSnapshot, error) {
	if len(components) == 0 {
		return BatchSnapshot{}, errors.New("recipe has no components")
	}
	totals := accumulateComponents(components)
	if totals.total <= 0 {
		return BatchSnapshot{}, errors.New("recipe has no mass")
	}

	snapshot := BatchSnapshot{
		TotalMassKg:        totals.total,
		WaterMassKg:        totals.water,
		FatMassKg:          totals.fat,
		ProteinMassKg:      totals.protein,
		Lactose:            totals.lactose,
		SucroseMassKg:      totals.sucrose,
		GlucoseMassKg:      totals.glucose,
		FructoseMassKg:     totals.fructose,
		MaltodextrinMassKg: totals.maltodextrin,
		PolyolsMassKg:      totals.polyols,
		AshMassKg:          totals.ash,
		OtherSolidsMassKg:  totals.other,
		TransFatMassKg:     totals.transFat,
		SaturatedFat:       totals.saturated,
		AddedSugars:        totals.added,
		EmulsifierPower:    totals.emulsifier,
		EmulsifierMassKg:   totals.emulsifierMass,
		BoundWaterKg:       totals.boundWater,
		PolymerSolidsKg:    totals.polymerSolids,
		ColligativeMoles:   totals.colligativeMoles,
		CholesterolMgTotal: totals.cholesterol,
		SweetnessEq:        totals.sweetness,
		CostTotal:          totals.cost,
		MixVolumeL:         totals.total / mixDensityKgPerL,
	}

	safeTotal := math.Max(1e-9, snapshot.TotalMassKg)
	snapshot.WaterPct = snapshot.WaterMassKg / safeTotal
	snapshot.SolidsPct = (snapshot.TotalMassKg - snapshot.WaterMassKg) / safeTotal
	snapshot.FatPct = snapshot.FatMassKg / safeTotal
	snapshot.ProteinPct = snapshot.ProteinMassKg / safeTotal
	snapshot.LactosePct = snapshot.Lactose.Mid / safeTotal
	snapshot.LactoseMinPct = snapshot.Lactose.Lo / safeTotal
	snapshot.LactoseMaxPct = snapshot.Lactose.Hi / safeTotal
	totalSugars := snapshot.SucroseMassKg + snapshot.GlucoseMassKg + snapshot.FructoseMassKg + snapshot.Lactose.Mid
	snapshot.TotalSugarsPct = totalSugars / safeTotal
	snapshot.AddedSugarsPct = snapshot.AddedSugars.Mid / safeTotal
	snapshot.AddedSugarsMinPct = snapshot.AddedSugars.Lo / safeTotal
	snapshot.AddedSugarsMaxPct = snapshot.AddedSugars.Hi / safeTotal
	snapshot.TransFatPct = snapshot.TransFatMassKg / safeTotal
	snapshot.SaturatedFatPct = snapshot.SaturatedFat.Mid / safeTotal
	snapshot.SaturatedFatMinPct = snapshot.SaturatedFat.Lo / safeTotal
	snapshot.SaturatedFatMaxPct = snapshot.SaturatedFat.Hi / safeTotal
	snapshot.CholesterolMgPerKg = snapshot.CholesterolMgTotal / safeTotal
	snapshot.PolymerSolidsPct = snapshot.PolymerSolidsKg / safeTotal
	snapshot.CostPerKg = snapshot.CostTotal / safeTotal

	return snapshot, nil
}

// FormulationBreakdown summarizes a batch snapshot into the lightweight
// Formulation struct used by analyses and reporting.
func (b BatchSnapshot) FormulationBreakdown() (Formulation, error) {
	batch := b.TotalMassKg
	if batch <= 0 {
		return Formulation{}, errors.New("snapshot has zero total mass")
	}

	sugars := map[string]float64{
		"sucrose":      b.SucroseMassKg / batch,
		"glucose":      b.GlucoseMassKg / batch,
		"fructose":     b.FructoseMassKg / batch,
		"lactose":      b.Lactose.Mid / batch,
		"polyols":      b.PolyolsMassKg / batch,
		"maltodextrin": b.MaltodextrinMassKg / batch,
	}

	snf := (b.ProteinMassKg + b.Lactose.Mid + b.AshMassKg) / batch
	stabilizer := b.PolymerSolidsKg / batch
	emulsifier := b.EmulsifierMassKg / batch

	return Formulation{
		MilkfatPct:    b.FatPct,
		SNFPct:        snf,
		WaterPct:      b.WaterPct,
		SugarsPct:     sugars,
		StabilizerPct: stabilizer,
		EmulsifierPct: emulsifier,
		ProteinPct:    b.ProteinPct,
	}, nil
}

// NutritionFactsSummary renders per-serving nutrition data from the snapshot.
func (b BatchSnapshot) NutritionFactsSummary(servingSizeGrams float64, sodiumMg float64) (NutritionFacts, error) {
	batch := b.TotalMassKg
	if batch <= 0 {
		return NutritionFacts{}, errors.New("snapshot has zero total mass")
	}

	fatPct := b.FatPct
	sugarsPct := b.TotalSugarsPct
	proteinPct := b.ProteinPct
	snfPct := (b.ProteinMassKg + b.Lactose.Mid + b.AshMassKg) / batch
	carbsPct := sugarsPct + snfPct - proteinPct
	transFatPct := b.TransFatPct
	saturatedFatPct := b.SaturatedFatPct
	addedSugarsPct := b.AddedSugarsPct

	fatG := fatPct * servingSizeGrams
	carbsG := carbsPct * servingSizeGrams
	proteinG := proteinPct * servingSizeGrams
	sugarsG := sugarsPct * servingSizeGrams
	transFatG := transFatPct * servingSizeGrams
	satFatG := saturatedFatPct * servingSizeGrams
	addedSugarsG := addedSugarsPct * servingSizeGrams
	cholMgPerKg := b.CholesterolMgPerKg
	cholMg := cholMgPerKg * (servingSizeGrams / 1000.0)
	calories := 9*fatG + 4*carbsG + 4*proteinG

	return NutritionFacts{
		ServingSizeGrams:   servingSizeGrams,
		Calories:           calories,
		TotalFatGrams:      fatG,
		TotalCarbGrams:     carbsG,
		TotalSugarsGrams:   sugarsG,
		ProteinGrams:       proteinG,
		SodiumMg:           sodiumMg,
		SaturatedFatGrams:  satFatG,
		SaturatedFatPct:    saturatedFatPct,
		TransFatGrams:      transFatG,
		TransFatPct:        transFatPct,
		AddedSugarsGrams:   addedSugarsG,
		AddedSugarsPct:     addedSugarsPct,
		FatPct:             fatPct,
		CarbsPct:           carbsPct,
		SugarsPct:          sugarsPct,
		ProteinPct:         proteinPct,
		CholesterolMg:      cholMg,
		CholesterolMgPerKg: cholMgPerKg,
	}, nil
}

// BuildProperties aggregates components and applies process calculations.
func BuildProperties(components []RecipeComponent, opts MixOptions) (BatchSnapshot, error) {
	snapshot, err := NewBatchSnapshot(components)
	if err != nil {
		return BatchSnapshot{}, err
	}
	snapshot.applyProcess(opts)
	return snapshot, nil
}

func (b *BatchSnapshot) applyProcess(opts MixOptions) {
	opts = defaultMixOptions(opts)
	if b.TotalMassKg <= 0 {
		return
	}
	safeTotal := math.Max(1e-9, b.TotalMassKg)
	waterAvailable := math.Max(1e-6, b.WaterMassKg-b.BoundWaterKg)
	mColligative := b.ColligativeMoles / waterAvailable
	b.FreezingPointC = -kfWater * mColligative

	absT := math.Abs(opts.ServeTempC)
	targetFreeWater := math.Max(1e-6, b.ColligativeMoles*kfWater/math.Max(1e-6, absT))
	targetFreeWater = math.Min(targetFreeWater, waterAvailable)
	b.IceFractionAtServe = math.Max(0.0, (waterAvailable-targetFreeWater)/math.Max(1e-6, b.WaterMassKg))

	polymerPct := math.Max(0, b.PolymerSolidsKg/safeTotal)
	muSerum := 0.0016 * math.Exp(0.045*(b.SolidsPct*100-36.0))
	polymerFactor := math.Exp(12.0 * polymerPct)
	tempFactor := math.Exp(0.025 * (5.0 - opts.ServeTempC))
	n := math.Max(0.55, 1.0-0.6*polymerPct*100)
	b.ViscosityAtServe = muSerum * polymerFactor * tempFactor * math.Pow(math.Max(1e-6, opts.ShearRate)/50.0, n-1.0)

	emulsifier := math.Max(0, b.EmulsifierPower/safeTotal)
	destab := (b.FatPct * 100.0) * (0.4 + emulsifier) / (4.0 + b.ProteinPct*100.0)
	viscTerm := 1.0 / (1.0 + math.Exp(6.5*(b.ViscosityAtServe-0.45)))
	fatTerm := 1.0 / (1.0 + math.Exp(-3.0*(destab-1.2)))
	overrun := math.Max(0.02, math.Min(1.1, 0.20+0.45*fatTerm+0.35*viscTerm))
	if opts.LimitOverrun {
		overrun = math.Min(overrun, opts.OverrunCap)
	}
	b.OverrunEstimate = overrun

	polyolsPct := b.PolyolsMassKg / safeTotal
	b.HardnessIndex = 30.0*b.IceFractionAtServe + 8.0*b.SolidsPct + 3.0*polyolsPct
	b.MeltdownIndex = math.Max(0.0, 1.2*b.SolidsPct+0.8*b.IceFractionAtServe+0.3*overrun-0.1*polyolsPct)

	solubility := 0.18 * math.Exp(0.012*opts.ServeTempC+1.2)
	availableWater := math.Max(1e-6, b.WaterMassKg-b.BoundWaterKg)
	lactoseConc := b.Lactose.Mid / math.Max(1e-6, availableWater)
	b.LactoseSupersaturation = lactoseConc / math.Max(1e-6, solubility)

	cp := 3.4 - 1.2*b.FatPct
	deltaT := 4.0 - opts.DrawTempC
	latent := 333.0 * b.IceFractionAtServe * b.WaterMassKg
	b.FreezerLoadKJ = cp*b.TotalMassKg*deltaT + latent

	b.PintsYield = b.MixVolumeL * (1 + overrun) / pintLiters
	if b.PintsYield > 0 {
		b.CostPerPint = b.CostTotal / b.PintsYield
	}
}

type batchTotals struct {
	total            float64
	water            float64
	fat              float64
	protein          float64
	ash              float64
	other            float64
	sucrose          float64
	glucose          float64
	fructose         float64
	maltodextrin     float64
	polyols          float64
	transFat         float64
	emulsifier       float64
	emulsifierMass   float64
	boundWater       float64
	polymerSolids    float64
	colligativeMoles float64
	cholesterol      float64
	added            IntervalMass
	lactose          IntervalMass
	saturated        IntervalMass
	sweetness        float64
	cost             float64
}

func accumulateComponents(components []RecipeComponent) batchTotals {
	var totals batchTotals
	for _, comp := range components {
		if comp.MassKg <= 0 {
			continue
		}
		profile := comp.Ingredient.EffectiveProfile()
		fractions := ConstituentsFromProfile(profile)
		mass := comp.MassKg

		totals.total += mass
		totals.water += mass * fractions.Water
		totals.fat += mass * fractions.Fat
		totals.protein += mass * fractions.Protein
		totals.ash += mass * fractions.Ash
		totals.other += mass * fractions.OtherSolids
		totals.sucrose += mass * fractions.Sucrose
		totals.glucose += mass * fractions.Glucose
		totals.fructose += mass * fractions.Fructose
		totals.maltodextrin += mass * fractions.Maltodextrin
		totals.polyols += mass * fractions.Polyols

		totals.lactose.add(profile.LactoseInterval(), mass)
		totals.added.add(profile.AddedSugarsInterval(), mass)
		totals.saturated.add(profile.Nutrition.SaturatedFat, mass)

		totals.transFat += mass * profile.Nutrition.TransFat.Mid()
		totals.cholesterol += mass * profile.Nutrition.CholesterolMgPerKg.Mid()
		totals.emulsifier += mass * profile.Functionals.EmulsifierPower.Mid()
		totals.boundWater += mass * profile.Functionals.WaterBinding.Mid()
		if profile.Functionals.Hydrocolloid {
			polymer := fractions.OtherSolids + fractions.Maltodextrin + fractions.Polyols
			totals.polymerSolids += mass * polymer
		}
		if profile.Functionals.EmulsifierPower.Mid() > 0 {
			totals.emulsifierMass += mass
		}
		totals.colligativeMoles += colligativeContribution(mass, fractions, profile.Functionals)
		totals.cost += mass * profile.Economics.Cost.Mid()
	}

	totals.sweetness = totals.sucrose*1.0 +
		totals.glucose*0.74 +
		totals.fructose*1.7 +
		totals.lactose.Mid*0.16 +
		totals.maltodextrin*0.20 +
		totals.polyols*0.60

	return totals
}

func colligativeContribution(mass float64, fractions ConstituentSet, funcs ConstituentFunctionals) float64 {
	maltodextrinMW := mwGlucose * math.Max(1.0, maltodextrinDP(funcs))
	polyMW := polyolMW(funcs)
	factor := osmoticFactor(funcs)
	moles := mass*fractions.Sucrose*1000.0/mwSucrose +
		mass*fractions.Glucose*1000.0/mwGlucose +
		mass*fractions.Fructose*1000.0/mwFructose +
		mass*fractions.Lactose*1000.0/mwLactose +
		mass*fractions.Maltodextrin*1000.0/maltodextrinMW +
		mass*fractions.Polyols*1000.0/math.Max(1e-6, polyMW)

	if funcs.EffectiveMW > 0 {
		moles += mass * fractions.OtherSolids * 1000.0 / funcs.EffectiveMW
	}
	return moles * factor
}
