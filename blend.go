package creamery

// BlendComponent couples an ingredient lot with a weight contribution.
type BlendComponent struct {
	Lot    LotDescriptor
	Weight float64
}

// Blend represents a normalized set of ingredient contributions.
type Blend struct {
	Components []BlendComponent
}

// TotalWeight returns the sum of component weights.
func (b Blend) TotalWeight() float64 {
	total := 0.0
	for _, comp := range b.Components {
		total += comp.Weight
	}
	return total
}

// AsFractions returns a copy of the blend scaled to sum to 1.
func (b Blend) AsFractions() Blend {
	total := b.TotalWeight()
	if total == 0 {
		return b
	}
	frac := make([]BlendComponent, 0, len(b.Components))
	inv := 1 / total
	for _, comp := range b.Components {
		if comp.Weight <= 0 {
			continue
		}
		frac = append(frac, BlendComponent{
			Lot:    comp.Lot,
			Weight: comp.Weight * inv,
		})
	}
	return Blend{Components: frac}
}

// WeightByID returns a map keyed by ingredient ID.
func (b Blend) WeightByID() map[IngredientID]float64 {
	weights := make(map[IngredientID]float64, len(b.Components))
	for _, comp := range b.Components {
		if comp.Weight <= 0 || comp.Lot.Definition == nil {
			continue
		}
		weights[comp.Lot.Definition.ID] += comp.Weight
	}
	return weights
}

// ToRecipeComponents scales blend fractions into concrete masses.
func (b Blend) ToRecipeComponents(totalMass float64) []RecipeComponent {
	if totalMass <= 0 {
		totalMass = 1
	}
	components := make([]RecipeComponent, 0, len(b.Components))
	for _, comp := range b.Components {
		if comp.Weight <= 0 {
			continue
		}
		components = append(components, RecipeComponent{
			Ingredient: comp.Lot,
			MassKg:     comp.Weight * totalMass,
		})
	}
	return components
}
