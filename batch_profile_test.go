package creamery

import "testing"

func TestBuildBatchProfileMatchesSweetenerAnalysis(t *testing.T) {
	cream := SpecFromComposition("Cream", PointComposition(0.36, 0.06, 0.05, 0.03))
	milk := SpecFromComposition("Milk", PointComposition(0.032, 0.09, 0.05, 0.01))
	sugar := SpecFromComposition("Sugar", PointComposition(0, 0, 1, 0))

	specs := []IngredientSpec{cream, milk, sugar}
	weights := map[IngredientID]float64{
		cream.ID: 0.4,
		milk.ID:  0.4,
		sugar.ID: 0.2,
	}

	profile := BuildBatchProfile(weights, specs, nil)
	sol := &Solution{Weights: weights}
	ref := AnalyzeSweeteners(sol, specs)

	if profile.Sweeteners.TotalPOD != ref.TotalPOD {
		t.Fatalf("total POD mismatch: got %.2f want %.2f", profile.Sweeteners.TotalPOD, ref.TotalPOD)
	}
	if profile.Sweeteners.TotalPAC != ref.TotalPAC {
		t.Fatalf("total PAC mismatch: got %.2f want %.2f", profile.Sweeteners.TotalPAC, ref.TotalPAC)
	}
}
