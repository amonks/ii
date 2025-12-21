package creamery

// BatchProfile aggregates constituent and sweetener data for a set of ingredient weights.
type BatchProfile struct {
	Components ConstituentComponents
	Sweeteners SweetenerAnalysis
}

// BuildBatchProfile accumulates constituent components and sweetener metrics for the given weights.
func BuildBatchProfile(weights map[IngredientID]float64, specs []IngredientSpec, lots map[IngredientID]IngredientLot) BatchProfile {
	var profile BatchProfile

	for _, spec := range specs {
		w := weights[spec.ID]
		if w <= 0 {
			continue
		}

		sourceProfile := spec.Profile
		if lot, ok := lots[spec.ID]; ok {
			sourceProfile = lot.EffectiveProfile()
		}

		accumulateProfile(&profile.Components, sourceProfile, w)
		addSweetenerContribution(&profile.Sweeteners, sourceProfile, w)
	}

	finalizeSweetenerTotals(&profile.Sweeteners)
	return profile
}

// BuildBatchProfileFromBlend aggregates components directly from a blend.
func BuildBatchProfileFromBlend(blend Blend) BatchProfile {
	var profile BatchProfile
	for _, comp := range blend.Components {
		if comp.Weight <= 0 {
			continue
		}
		sourceProfile := comp.Lot.EffectiveProfile()
		accumulateProfile(&profile.Components, sourceProfile, comp.Weight)
		addSweetenerContribution(&profile.Sweeteners, sourceProfile, comp.Weight)
	}
	finalizeSweetenerTotals(&profile.Sweeteners)
	return profile
}

func accumulateProfile(total *ConstituentComponents, profile ConstituentProfile, weight float64) {
	if weight <= 0 {
		return
	}
	comps := profile.Components
	total.Water = total.Water.Add(comps.Water.Scale(weight))
	total.Fat = total.Fat.Add(comps.Fat.Scale(weight))
	total.MSNF = total.MSNF.Add(profile.MSNFInterval().Scale(weight))
	total.Protein = total.Protein.Add(comps.Protein.Scale(weight))
	total.Lactose = total.Lactose.Add(comps.Lactose.Scale(weight))
	total.Sucrose = total.Sucrose.Add(comps.Sucrose.Scale(weight))
	total.Glucose = total.Glucose.Add(comps.Glucose.Scale(weight))
	total.Fructose = total.Fructose.Add(comps.Fructose.Scale(weight))
	total.Maltodextrin = total.Maltodextrin.Add(comps.Maltodextrin.Scale(weight))
	total.Polyols = total.Polyols.Add(comps.Polyols.Scale(weight))
	total.Ash = total.Ash.Add(comps.Ash.Scale(weight))
	total.OtherSolids = total.OtherSolids.Add(comps.OtherSolids.Scale(weight))
}

func addSweetenerContribution(analysis *SweetenerAnalysis, profile ConstituentProfile, weight float64) {
	if weight <= 0 {
		return
	}
	analysis.AddedSugarPOD += weight * profile.AddedPODInterval().Mid()
	analysis.LactosePOD += weight * profile.LactosePODInterval().Mid()
	analysis.AddedSugarPAC += weight * profile.AddedPACInterval().Mid()
	analysis.LactosePAC += weight * profile.LactosePACInterval().Mid()
}

func finalizeSweetenerTotals(analysis *SweetenerAnalysis) {
	analysis.TotalPOD = analysis.AddedSugarPOD + analysis.LactosePOD
	analysis.TotalPAC = analysis.AddedSugarPAC + analysis.LactosePAC
}
