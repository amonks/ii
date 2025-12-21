package creamery

func makeSpecFromFractions(name string, fractions ComponentFractions) IngredientDefinition {
	comps := EnsureWater(fractions)
	profile := ConstituentProfile{
		ID:         NewIngredientID(name),
		Name:       name,
		Components: comps,
	}
	return SpecFromProfile(profile)
}
