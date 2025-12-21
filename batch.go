package creamery

// Batch couples a normalized blend with a concrete total mass for analysis.
type Batch struct {
	Blend       Blend
	TotalMassKg float64
}

// NewBatch normalizes the provided blend and enforces a positive total mass.
func NewBatch(blend Blend, totalMassKg float64) Batch {
	normalized := blend.AsFractions()
	if totalMassKg <= 0 {
		totalMassKg = 1
	}
	return Batch{
		Blend:       normalized,
		TotalMassKg: totalMassKg,
	}
}

// BatchFromPortions constructs a batch from raw portion data.
func BatchFromPortions(portions []Portion, totalMassKg float64) Batch {
	return NewBatch(Blend{Components: portions}, totalMassKg)
}

// PortionMasses expands the normalized blend into absolute masses.
func (b Batch) PortionMasses() []PortionMass {
	return PortionsToMasses(b.Blend.Components, b.TotalMassKg)
}

// Components returns recipe components derived from the batch.
func (b Batch) Components() []RecipeComponent {
	masses := b.PortionMasses()
	components := make([]RecipeComponent, 0, len(masses))
	for _, mass := range masses {
		if mass.MassKg <= 0 {
			continue
		}
		components = append(components, RecipeComponent{
			Ingredient: mass.Lot,
			MassKg:     mass.MassKg,
		})
	}
	return components
}

// FractionsByName aggregates normalized fractions keyed by display name.
func (b Batch) FractionsByName() map[string]float64 {
	fractions := make(map[string]float64, len(b.Blend.Components))
	for _, portion := range b.Blend.Components {
		if portion.Fraction <= 0 {
			continue
		}
		name := portion.Lot.DisplayName()
		fractions[name] += portion.Fraction
	}
	return fractions
}

// FractionsByID aggregates normalized fractions keyed by ingredient ID.
func (b Batch) FractionsByID() map[IngredientID]float64 {
	fractions := make(map[IngredientID]float64, len(b.Blend.Components))
	for _, portion := range b.Blend.Components {
		if portion.Fraction <= 0 || portion.Lot.Definition == nil {
			continue
		}
		fractions[portion.Lot.Definition.ID] += portion.Fraction
	}
	return fractions
}

// Snapshot aggregates the batch into component totals without process options.
func (b Batch) Snapshot() (BatchSnapshot, error) {
	return NewBatchSnapshot(b.Components())
}
