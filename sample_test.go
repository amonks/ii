package creamery

import "testing"

func TestBestSamplePrefersHighestViscosityScore(t *testing.T) {
	gel := makeHydrocolloidSpec("Hydro Base", 0.12, true)
	diluent := makeHydrocolloidSpec("Diluent", 0.02, false)
	targetFractions := EnsureWater(ComponentFractions{
		Water:       Range(0.85, 0.99),
		OtherSolids: Range(0.01, 0.15),
	})
	target := FormulationTarget{Components: targetFractions}

	solver, err := NewSolver(NewProblem([]Ingredient{gel, diluent}, target))
	if err != nil {
		t.Fatalf("solver creation failed: %v", err)
	}

	best, err := solver.Optimize(nil)
	if err != nil {
		t.Fatalf("BestSample failed: %v", err)
	}

	gelWeight := best.Weights[gel.ID]
	diluentWeight := best.Weights[diluent.ID]
	if gelWeight < diluentWeight {
		t.Fatalf("expected hydro base weight >= diluent weight (%.3f vs %.3f)", gelWeight, diluentWeight)
	}

	bestScore, err := best.Score(RecipePreference{}, MixOptions{})
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}

	extremes, err := solver.ExtremePoints()
	if err != nil || len(extremes) == 0 {
		t.Fatalf("extreme points failed: %v", err)
	}
	maxExtremeScore := -1.0
	for _, sol := range extremes {
		score, scoreErr := sol.Score(RecipePreference{}, MixOptions{})
		if scoreErr != nil {
			t.Fatalf("score failed: %v", scoreErr)
		}
		if score > maxExtremeScore {
			maxExtremeScore = score
		}
	}
	if bestScore+1e-6 < maxExtremeScore {
		t.Fatalf("expected best score %.4f to match/exceed extreme max %.4f", bestScore, maxExtremeScore)
	}

	comp, err := CompareSolutions(best, extremes[0], RecipePreference{}, MixOptions{})
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	if comp < 0 {
		t.Fatalf("expected best to rank >= first extreme, got %d", comp)
	}
}

func makeHydrocolloidSpec(name string, otherSolids float64, hydro bool) Ingredient {
	fractions := EnsureWater(ComponentFractions{
		Water:       Point(1 - otherSolids),
		OtherSolids: Point(otherSolids),
	})
	profile := ConstituentProfile{
		ID:         NewIngredientID(name),
		Name:       name,
		Components: fractions,
		Functionals: ConstituentFunctionals{
			Hydrocolloid: hydro,
		},
	}
	return SpecFromProfile(profile)
}
