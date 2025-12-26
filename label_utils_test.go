package creamery

import (
	"math"
	"testing"
)

func TestRecipeFromSolutionUsesIngredientIDs(t *testing.T) {
	spec := Ingredient{
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
		Lots: make(map[IngredientID]Lot),
	}

	goals := LabelGoals{
		BatchMassKG: 100,
		Overrun:     0,
	}

	inst := spec.DefaultLot()
	inst.Label = "stock_sucrose"
	sol.Lots[IngredientID("sucrose")] = inst

	recipe, _, _, _, _, err := recipeFromSolution(sol, []Ingredient{spec}, goals, 0)
	if err != nil {
		t.Fatalf("recipeFromSolution returned error: %v", err)
	}

	masses := recipe.MassComponents()
	if len(masses) != 1 {
		t.Fatalf("expected 1 component, got %d", len(masses))
	}
	if masses[0].Ingredient.DisplayName() != inst.DisplayName() {
		t.Fatalf("expected ingredient %s, got %s", inst.DisplayName(), masses[0].Ingredient.DisplayName())
	}
	if masses[0].MassKg != 50 {
		t.Fatalf("expected mass 50kg, got %f", masses[0].MassKg)
	}
}

func TestRecipeFromSolutionPreservesFractions(t *testing.T) {
	sucrose := Ingredient{
		ID:   IngredientID("sucrose"),
		Name: "Sucrose",
		Profile: ConstituentProfile{
			ID:   IngredientID("sucrose"),
			Name: "Sucrose",
			Components: ConstituentComponents{
				Sucrose: Point(1.0),
			},
		},
	}
	cream := Ingredient{
		ID:   IngredientID("cream"),
		Name: "Cream",
		Profile: ConstituentProfile{
			ID:   IngredientID("cream"),
			Name: "Cream",
			Components: ConstituentComponents{
				Fat:   Point(0.36),
				Water: Point(0.64),
			},
		},
	}

	specs := []Ingredient{sucrose, cream}
	sol := &Solution{
		Weights: map[IngredientID]float64{
			sucrose.ID: 0.6,
			cream.ID:   0.4,
		},
		Lots: map[IngredientID]Lot{
			sucrose.ID: sucrose.DefaultLot(),
			cream.ID:   cream.DefaultLot(),
		},
	}

	goals := LabelGoals{
		BatchMassKG: 80,
		Overrun:     0,
	}

	recipe, _, _, _, _, err := recipeFromSolution(sol, specs, goals, 0)
	if err != nil {
		t.Fatalf("recipeFromSolution returned error: %v", err)
	}
	if recipe == nil {
		t.Fatal("expected recipe")
	}

	fractions := make(map[IngredientID]float64)
	for _, portion := range recipe.Portions {
		if portion.Lot.Definition == nil {
			continue
		}
		fractions[portion.Lot.Definition.ID] += portion.Fraction
	}

	for id, expected := range sol.Weights {
		got := fractions[id]
		if math.Abs(got-expected) > 1e-6 {
			t.Fatalf("fraction mismatch for %s: got %.6f want %.6f", id, got, expected)
		}
	}
}
