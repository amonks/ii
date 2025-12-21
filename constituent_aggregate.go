package creamery

// accumulateProfile applies a mass weight to a constituent profile and adds
// the contribution into the running component totals.
func accumulateProfile(total *ConstituentComponents, profile ConstituentProfile, weight float64) {
	if total == nil || weight <= 0 {
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
