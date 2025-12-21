package creamery

import "fmt"

func recipeFromSolution(sol *Solution, specs []IngredientDefinition, goals LabelGoals, sodiumMg float64) (*Recipe, NutritionFacts, float64, BatchSnapshot, error) {
	if sol == nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, fmt.Errorf("nil solution")
	}

	batchMass := goals.BatchMassKG
	if batchMass <= 0 {
		batchMass = 100
	}

	components, err := componentsFromSolution(sol, specs, batchMass)
	if err != nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, err
	}

	recipe, err := NewRecipe(components, goals.Overrun)
	if err != nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, err
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

	snapshotMetrics, err := BuildProperties(components, opts)
	if err != nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, err
	}

	if snapshotMetrics.OverrunEstimate > 0 {
		if updated, err := recipe.WithOverrun(snapshotMetrics.OverrunEstimate); err == nil {
			recipe = &updated
		}
	}

	snapshot := ProductionSettings{
		ServeTempC: opts.ServeTempC,
		DrawTempC:  opts.DrawTempC,
		ShearRate:  opts.ShearRate,
		Snapshot:   snapshotMetrics,
	}
	snapRecipe := recipe.WithMixSnapshot(&snapshot)
	recipe = &snapRecipe

	servingSize, err := recipe.ServingSizeForVolume(servingPortionLiters, opts)
	if err != nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, err
	}

	facts, err := recipe.NutritionFacts(servingSize, sodiumMg)
	if err != nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, err
	}

	return recipe, facts, servingSize, snapshotMetrics, nil
}
