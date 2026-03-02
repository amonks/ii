package creamery

func makeSpecFromFractions(name string, fractions CompositionRange) Ingredient {
	comps := EnsureWater(fractions)
	profile := ConstituentProfile{
		ID:         NewIngredientID(name),
		Name:       name,
		Components: comps,
	}
	return SpecFromProfile(profile)
}
