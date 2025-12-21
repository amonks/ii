package linear

import "math"

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

func componentSums(keys []string, weights []float64, ingredients map[string]DetailedIngredient) map[string]float64 {
	totals := map[string]float64{
		"total":             0,
		"water":             0,
		"fat":               0,
		"trans_fat":         0,
		"saturated_fat":     0,
		"saturated_fat_min": 0,
		"saturated_fat_max": 0,
		"protein":           0,
		"lactose":           0,
		"lactose_min":       0,
		"lactose_max":       0,
		"sucrose":           0,
		"glucose":           0,
		"fructose":          0,
		"maltodextrin":      0,
		"polyols":           0,
		"ash":               0,
		"other_solids":      0,
		"emulsifier_power":  0,
		"bound_water":       0,
		"polymer_solids":    0,
		"colligative_moles": 0,
		"cholesterol_mg":    0,
		"added_sugars":      0,
		"added_sugars_min":  0,
		"added_sugars_max":  0,
	}

	for i, key := range keys {
		weight := weights[i]
		ing := ingredients[key]

		totals["total"] += weight
		totals["water"] += weight * ing.Water
		totals["fat"] += weight * ing.Fat
		totals["trans_fat"] += weight * ing.TransFat

		sat := ing.SaturatedFat
		if sat == 0 && ing.Fat > 0 {
			sat = ing.Fat
		}
		satMin := ing.SaturatedFatMin
		if satMin == 0 {
			satMin = sat
		}
		satMax := ing.SaturatedFatMax
		if satMax == 0 {
			satMax = sat
		}
		totals["saturated_fat"] += weight * sat
		totals["saturated_fat_min"] += weight * satMin
		totals["saturated_fat_max"] += weight * satMax

		totals["protein"] += weight * ing.Protein

		lact := ing.Lactose
		lactMin := ing.LactoseMin
		if lactMin == 0 {
			lactMin = lact
		}
		lactMax := ing.LactoseMax
		if lactMax == 0 {
			lactMax = lact
		}
		totals["lactose"] += weight * lact
		totals["lactose_min"] += weight * lactMin
		totals["lactose_max"] += weight * lactMax

		totals["sucrose"] += weight * ing.Sucrose
		totals["glucose"] += weight * ing.Glucose
		totals["fructose"] += weight * ing.Fructose
		totals["maltodextrin"] += weight * ing.Maltodextrin
		totals["polyols"] += weight * ing.Polyols
		totals["ash"] += weight * ing.Ash
		totals["other_solids"] += weight * ing.OtherSolids
		totals["emulsifier_power"] += weight * ing.EmulsifierPower
		totals["bound_water"] += weight * ing.WaterBinding

		if ing.Hydrocolloid {
			totals["polymer_solids"] += weight * (ing.OtherSolids + ing.Maltodextrin + ing.Polyols)
		}

		if ing.CholesterolMgPerKg > 0 {
			totals["cholesterol_mg"] += weight * ing.CholesterolMgPerKg
		}

		added := ing.AddedSugars
		addedMin := ing.AddedSugarsMin
		addedMax := ing.AddedSugarsMax
		totals["added_sugars"] += weight * added
		totals["added_sugars_min"] += weight * addedMin
		totals["added_sugars_max"] += weight * addedMax

		maltodextrinMW := mwGlucose * math.Max(1.0, ing.MaltodextrinDP)
		moles :=
			weight*ing.Sucrose*1000.0/mwSucrose +
				weight*ing.Glucose*1000.0/mwGlucose +
				weight*ing.Fructose*1000.0/mwFructose +
				weight*ing.Lactose*1000.0/mwLactose +
				weight*ing.Maltodextrin*1000.0/maltodextrinMW +
				weight*ing.Polyols*1000.0/math.Max(1e-6, ing.PolyolMW)

		polymerMoles := 0.0
		if ing.EffectiveMW > 0 {
			polymerMoles = weight * ing.OtherSolids * 1000.0 / ing.EffectiveMW
		}

		totals["colligative_moles"] += (moles + polymerMoles) * ing.OsmoticCoeff * ing.VHFactor
	}

	return totals
}

func sweetnessEq(totals map[string]float64) float64 {
	return totals["sucrose"]*1.0 +
		totals["glucose"]*0.74 +
		totals["fructose"]*1.7 +
		totals["lactose"]*0.16 +
		totals["maltodextrin"]*0.20 +
		totals["polyols"]*0.60
}

func freezingPointAndIceFraction(totals map[string]float64, tempC float64) (float64, float64) {
	waterAvailable := math.Max(1e-6, totals["water"]-totals["bound_water"])
	mColligative := totals["colligative_moles"] / waterAvailable
	freezingPoint := -kfWater * mColligative

	absT := math.Abs(tempC)
	targetFreeWater := math.Max(1e-6, totals["colligative_moles"]*kfWater/math.Max(1e-6, absT))
	targetFreeWater = math.Min(targetFreeWater, waterAvailable)
	iceFraction := math.Max(0.0, (waterAvailable-targetFreeWater)/math.Max(1e-6, totals["water"]))
	return freezingPoint, iceFraction
}

func viscosity(totals map[string]float64, tempC, shearRate float64) float64 {
	total := math.Max(1e-9, totals["total"])
	solidsPct := (totals["total"] - totals["water"]) / total
	polymerPct := totals["polymer_solids"] / total

	muSerum := 0.0016 * math.Exp(0.045*(solidsPct*100-36.0))
	polymerFactor := math.Exp(12.0 * polymerPct)
	tempFactor := math.Exp(0.025 * (5.0 - tempC))
	n := math.Max(0.55, 1.0-0.6*polymerPct*100)
	return muSerum * polymerFactor * tempFactor * math.Pow(math.Max(1e-6, shearRate)/50.0, n-1.0)
}

func estimateOverrun(totals map[string]float64, viscosityValue float64, cap float64, limit bool) float64 {
	total := math.Max(1e-9, totals["total"])
	fatPct := totals["fat"] / total
	protein := totals["protein"] / total
	emulsifier := totals["emulsifier_power"] / total
	destab := (fatPct * 100.0) * (0.4 + emulsifier) / (4.0 + protein*100.0)

	viscTerm := 1.0 / (1.0 + math.Exp(6.5*(viscosityValue-0.45)))
	fatTerm := 1.0 / (1.0 + math.Exp(-3.0*(destab-1.2)))
	raw := math.Max(0.02, math.Min(1.1, 0.20+0.45*fatTerm+0.35*viscTerm))
	if limit {
		return math.Min(raw, cap)
	}
	return raw
}

func hardnessMeltdown(iceFraction, solidsPct, polyols, overrunValue float64) (float64, float64) {
	hardness := 30.0*iceFraction + 8.0*solidsPct + 3.0*polyols
	meltdown := math.Max(0.0, 1.2*solidsPct+0.8*iceFraction+0.3*overrunValue-0.1*polyols)
	return hardness, meltdown
}

func lactoseSupersaturation(totals map[string]float64, tempC float64) float64 {
	solubility := 0.18 * math.Exp(0.012*tempC+1.2)
	availableWater := math.Max(1e-6, totals["water"]-totals["bound_water"])
	lactConc := totals["lactose"] / availableWater
	return lactConc / math.Max(1e-6, solubility)
}

func freezerLoad(totals map[string]float64, drawTemp float64, iceFraction float64) float64 {
	cp := 3.4 - 1.2*(totals["fat"]/math.Max(1e-6, totals["total"]))
	deltaT := 4.0 - drawTemp
	latent := 333.0 * iceFraction * totals["water"]
	return cp*totals["total"]*deltaT + latent
}

// BuildProperties replicates the Python build_properties helper.
func BuildProperties(keys []string, weights []float64, ingredients map[string]DetailedIngredient, opts MixOptions) map[string]float64 {
	opts = defaultMixOptions(opts)
	totals := componentSums(keys, weights, ingredients)
	safeTotal := math.Max(1e-9, totals["total"])
	solids := totals["total"] - totals["water"]
	fatPct := totals["fat"] / safeTotal
	proteinPct := totals["protein"] / safeTotal
	waterPct := totals["water"] / safeTotal
	totalSugars := totals["sucrose"] + totals["glucose"] + totals["fructose"] + totals["lactose"]
	totalSugarsPct := totalSugars / safeTotal
	sweetness := sweetnessEq(totals)
	transFatPct := totals["trans_fat"] / safeTotal
	saturatedFatPct := totals["saturated_fat"] / safeTotal
	saturatedFatMinPct := totals["saturated_fat_min"] / safeTotal
	saturatedFatMaxPct := totals["saturated_fat_max"] / safeTotal
	lactosePct := totals["lactose"] / safeTotal
	lactoseMinPct := totals["lactose_min"] / safeTotal
	lactoseMaxPct := totals["lactose_max"] / safeTotal
	addedSugarsPct := totals["added_sugars"] / safeTotal
	addedSugarsMinPct := totals["added_sugars_min"] / safeTotal
	addedSugarsMaxPct := totals["added_sugars_max"] / safeTotal
	cholesterolMgPerKg := totals["cholesterol_mg"] / safeTotal

	freezingPoint, iceFraction := freezingPointAndIceFraction(totals, opts.ServeTempC)
	viscosityValue := viscosity(totals, opts.ServeTempC, opts.ShearRate)
	overrunValue := estimateOverrun(totals, viscosityValue, opts.OverrunCap, opts.LimitOverrun)
	hardness, meltdown := hardnessMeltdown(
		iceFraction,
		solids/math.Max(1e-6, totals["total"]),
		totals["polyols"]/math.Max(1e-6, totals["total"]),
		overrunValue,
	)
	lactoseSS := lactoseSupersaturation(totals, opts.ServeTempC)
	load := freezerLoad(totals, opts.DrawTempC, iceFraction)
	polymerPct := totals["polymer_solids"] / safeTotal

	costTotal := 0.0
	for i, key := range keys {
		costTotal += weights[i] * ingredients[key].Cost
	}
	costPerKg := 0.0
	if totals["total"] > 0 {
		costPerKg = costTotal / totals["total"]
	}

	mixVolumeL := totals["total"] / mixDensityKgPerL
	pintsOut := mixVolumeL * (1 + overrunValue) / pintLiters
	costPerPint := 0.0
	if pintsOut > 0 {
		costPerPint = costTotal / pintsOut
	}

	return map[string]float64{
		"total_mass":              totals["total"],
		"water":                   totals["water"],
		"bound_water":             totals["bound_water"],
		"fat":                     totals["fat"],
		"fat_pct":                 fatPct,
		"trans_fat":               totals["trans_fat"],
		"trans_fat_pct":           transFatPct,
		"saturated_fat":           totals["saturated_fat"],
		"saturated_fat_pct":       saturatedFatPct,
		"saturated_fat_min":       totals["saturated_fat_min"],
		"saturated_fat_min_pct":   saturatedFatMinPct,
		"saturated_fat_max":       totals["saturated_fat_max"],
		"saturated_fat_max_pct":   saturatedFatMaxPct,
		"protein":                 totals["protein"],
		"protein_pct":             proteinPct,
		"water_pct":               waterPct,
		"solids_pct":              solids / safeTotal,
		"total_sugars":            totalSugars,
		"total_sugars_pct":        totalSugarsPct,
		"lactose":                 totals["lactose"],
		"lactose_pct":             lactosePct,
		"lactose_min":             totals["lactose_min"],
		"lactose_min_pct":         lactoseMinPct,
		"lactose_max":             totals["lactose_max"],
		"lactose_max_pct":         lactoseMaxPct,
		"solids":                  solids,
		"sweetness_eq":            sweetness,
		"freezing_point":          freezingPoint,
		"ice_fraction_at_serve":   iceFraction,
		"viscosity_at_serve":      viscosityValue,
		"overrun_estimate":        overrunValue,
		"hardness_index":          hardness,
		"meltdown_index":          meltdown,
		"lactose_supersaturation": lactoseSS,
		"freezer_load_kj":         load,
		"polymer_solids_pct":      polymerPct,
		"cholesterol_mg_total":    totals["cholesterol_mg"],
		"cholesterol_mg_per_kg":   cholesterolMgPerKg,
		"added_sugars":            totals["added_sugars"],
		"added_sugars_pct":        addedSugarsPct,
		"added_sugars_min":        totals["added_sugars_min"],
		"added_sugars_min_pct":    addedSugarsMinPct,
		"added_sugars_max":        totals["added_sugars_max"],
		"added_sugars_max_pct":    addedSugarsMaxPct,
		"cost_total":              costTotal,
		"cost_per_kg":             costPerKg,
		"volume_L":                mixVolumeL,
		"pints_yield":             pintsOut,
		"cost_per_pint_overrun":   costPerPint,
	}
}
