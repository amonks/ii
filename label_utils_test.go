package creamery

import "testing"

func TestRecipeFromSolutionUsesIngredientIDs(t *testing.T) {
	spec := IngredientSpec{
		ID:   IngredientID("sucrose"),
		Name: "Fancy Sugar",
		Profile: ConstituentProfile{
			ID:   IngredientID("sucrose"),
			Name: "Fancy Sugar",
			Components: ConstituentComponents{
				Sucrose:      Point(1.0),
				Water:        Point(0),
				Fat:          Point(0),
				Protein:      Point(0),
				Lactose:      Point(0),
				OtherSolids:  Point(0),
				Glucose:      Point(0),
				Fructose:     Point(0),
				Maltodextrin: Point(0),
				Polyols:      Point(0),
			},
		},
	}

	sol := &Solution{
		Weights: map[IngredientID]float64{
			IngredientID("sucrose"): 0.5,
		},
		Names: map[IngredientID]string{
			IngredientID("sucrose"): "Fancy Sugar",
		},
		Lots: make(map[IngredientID]IngredientLot),
	}

	goals := LabelGoals{
		BatchMassKG: 100,
		Overrun:     0,
	}

	inst := NewIngredientLot(spec)
	inst.Name = "stock_sucrose"
	sol.Lots[IngredientID("sucrose")] = inst

	recipe, _, _, _, err := recipeFromSolution(sol, []IngredientSpec{spec}, goals, 0)
	if err != nil {
		t.Fatalf("recipeFromSolution returned error: %v", err)
	}

	if len(recipe.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(recipe.Components))
	}
	if recipe.Components[0].Ingredient.Name != inst.Name {
		t.Fatalf("expected ingredient %s, got %s", inst.Name, recipe.Components[0].Ingredient.Name)
	}
	if recipe.Components[0].MassKg != 50 {
		t.Fatalf("expected mass 50kg, got %f", recipe.Components[0].MassKg)
	}
}
