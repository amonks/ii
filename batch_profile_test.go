package creamery

import "testing"

func TestBuildBatchProfileMatchesSweetenerAnalysis(t *testing.T) {
	cream := makeSpecFromFractions("Cream", ComponentFractions{
		Fat:         Point(0.36),
		MSNF:        Point(0.06),
		Sucrose:     Point(0.05),
		OtherSolids: Point(0.03),
	})
	milk := makeSpecFromFractions("Milk", ComponentFractions{
		Fat:         Point(0.032),
		MSNF:        Point(0.09),
		Sucrose:     Point(0.05),
		OtherSolids: Point(0.01),
	})
	sugar := makeSpecFromFractions("Sugar", ComponentFractions{
		Sucrose: Point(1),
	})

	specs := []IngredientDefinition{cream, milk, sugar}
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
