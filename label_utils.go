package creamery

import "fmt"

func recipeFromSolution(sol *Solution, specs []Ingredient, goals LabelGoals, sodiumMg float64) (*Recipe, NutritionFacts, float64, BatchSnapshot, ProcessProperties, error) {
	if sol == nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, ProcessProperties{}, fmt.Errorf("nil solution")
	}

	batchMass := goals.BatchMassKG
	if batchMass <= 0 {
		batchMass = 100
	}

	components, err := componentsFromSolution(sol, specs, batchMass)
	if err != nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, ProcessProperties{}, err
	}

	recipe, err := NewRecipe(components, goals.Overrun)
	if err != nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, ProcessProperties{}, err
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

	snapshotMetrics, processProps, err := BuildProperties(components, opts)
	if err != nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, ProcessProperties{}, err
	}

	if processProps.OverrunEstimate > 0 {
		if updated, err := recipe.WithOverrun(processProps.OverrunEstimate); err == nil {
			recipe = &updated
		}
	}

	snapshot := ProductionSettings{
		MixOptions: opts,
		Snapshot:   snapshotMetrics,
	}
	snapRecipe := recipe.WithMixSnapshot(&snapshot)
	recipe = &snapRecipe

	servingSize, err := recipe.ServingSizeForVolume(servingPortionLiters, opts)
	if err != nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, ProcessProperties{}, err
	}

	facts, err := recipe.NutritionFacts(servingSize, sodiumMg)
	if err != nil {
		return nil, NutritionFacts{}, 0, BatchSnapshot{}, ProcessProperties{}, err
	}

	return recipe, facts, servingSize, snapshotMetrics, processProps, nil
}
