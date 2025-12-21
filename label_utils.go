package creamery

import "fmt"

func recipeFromSolution(sol *Solution, ingredients []Ingredient, detailed map[string]DetailedIngredient, goals LabelGoals, sodiumMg float64) (*Recipe, NutritionFacts, float64, map[string]float64, error) {
	if sol == nil {
		return nil, NutritionFacts{}, 0, nil, fmt.Errorf("nil solution")
	}

	keys := make([]string, 0, len(ingredients))
	weights := make([]float64, 0, len(ingredients))
	table := make(map[string]DetailedIngredient, len(ingredients))
	components := make([]RecipeComponent, 0, len(ingredients))

	for _, ing := range ingredients {
		w := sol.Weights[ing.Name]
		if w <= 1e-6 {
			continue
		}
		detail, ok := detailed[ing.Name]
		if !ok {
			return nil, NutritionFacts{}, 0, nil, fmt.Errorf("missing detailed composition for %s", ing.Name)
		}
		mass := w * goals.BatchMassKG
		entry := detail
		keys = append(keys, ing.Name)
		weights = append(weights, mass)
		table[ing.Name] = entry
		components = append(components, RecipeComponent{
			Ingredient: &entry,
			MassKg:     mass,
		})
	}

	recipe, err := NewRecipe(components, goals.Overrun)
	if err != nil {
		return nil, NutritionFacts{}, 0, nil, err
	}

	opts := MixOptions{
		ServeTempC: goals.ServeTemperature,
		DrawTempC:  goals.DrawTemperature,
		ShearRate:  goals.ShearRate,
	}
	if goals.OverrunCap != nil {
		opts.OverrunCap = *goals.OverrunCap
		opts.LimitOverrun = true
	}

	metrics := BuildProperties(keys, weights, table, opts)

	if val, ok := metrics["overrun_estimate"]; ok {
		if updated, err := recipe.WithOverrun(val); err == nil {
			recipe = &updated
		}
	}

	snapshot := ProductionSettings{
		ServeTempC: opts.ServeTempC,
		DrawTempC:  opts.DrawTempC,
		ShearRate:  opts.ShearRate,
		Metrics:    metrics,
	}
	snapRecipe := recipe.WithMixSnapshot(&snapshot)
	recipe = &snapRecipe

	servingSize, err := recipe.ServingSizeForVolume(servingPortionLiters, opts)
	if err != nil {
		return nil, NutritionFacts{}, 0, nil, err
	}

	facts, err := recipe.NutritionFacts(servingSize, sodiumMg)
	if err != nil {
		return nil, NutritionFacts{}, 0, nil, err
	}

	return recipe, facts, servingSize, metrics, nil
}
