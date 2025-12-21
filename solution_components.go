package creamery

import "fmt"

// componentsFromSolution converts a solver solution into concrete recipe
// components scaled to the requested batch mass (kg). When the solution
// already provides a normalized blend, that takes precedence; otherwise
// the function falls back to the explicit weight map.
func componentsFromSolution(sol *Solution, specs []IngredientDefinition, batchMass float64) ([]RecipeComponent, error) {
	if sol == nil {
		return nil, fmt.Errorf("nil solution")
	}
	if batchMass <= 0 {
		batchMass = 100
	}

	components := make([]RecipeComponent, 0, len(specs))
	if len(sol.Blend.Components) > 0 {
		batch := NewBatch(sol.Blend, batchMass)
		return batch.Components(), nil
	}

	if len(specs) == 0 && len(sol.Lots) > 0 {
		for id, lot := range sol.Lots {
			weight := sol.Weights[id]
			if weight <= 1e-6 {
				continue
			}
			components = append(components, RecipeComponent{
				Ingredient: lot,
				MassKg:     weight * batchMass,
			})
		}
		return components, nil
	}

	for _, spec := range specs {
		weight := sol.Weights[spec.ID]
		if weight <= 1e-6 {
			continue
		}
		lot, ok := sol.Lots[spec.ID]
		if !ok {
			lot = spec.DefaultLot()
		}
		components = append(components, RecipeComponent{
			Ingredient: lot,
			MassKg:     weight * batchMass,
		})
	}

	return components, nil
}
