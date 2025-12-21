package creamery

// Blend represents a normalized set of ingredient contributions.
type Blend struct {
	Components []Portion
}

// TotalFraction returns the sum of component fractions.
func (b Blend) TotalFraction() float64 {
	total := 0.0
	for _, comp := range b.Components {
		total += comp.Fraction
	}
	return total
}

// AsFractions returns a normalized copy of the blend.
func (b Blend) AsFractions() Blend {
	return Blend{Components: NormalizePortions(b.Components)}
}

// FractionByID returns a map keyed by ingredient ID.
func (b Blend) FractionByID() map[IngredientID]float64 {
	weights := make(map[IngredientID]float64, len(b.Components))
	for _, comp := range b.Components {
		if comp.Fraction <= 0 || comp.Lot.Definition == nil {
			continue
		}
		weights[comp.Lot.Definition.ID] += comp.Fraction
	}
	return weights
}

// ToRecipeComponents scales blend fractions into concrete masses.
func (b Blend) ToRecipeComponents(totalMass float64) []RecipeComponent {
	masses := PortionsToMasses(b.Components, totalMass)
	components := make([]RecipeComponent, 0, len(masses))
	for _, mass := range masses {
		components = append(components, RecipeComponent{
			Ingredient: mass.Lot,
			MassKg:     mass.MassKg,
		})
	}
	return components
}
