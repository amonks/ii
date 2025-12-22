package creamery

import "testing"

func TestViscosityPreferenceScore(t *testing.T) {
	pref := DefaultViscosityPreference()
	center := (pref.Lower + pref.Upper) / 2
	lowerPenalty := pref.Score(pref.Lower - pref.Transition*2)
	upperPenalty := pref.Score(pref.Upper + pref.Transition*2)
	centerScore := pref.Score(center)

	if centerScore <= lowerPenalty {
		t.Fatalf("expected center score %.3f to exceed lower penalty %.3f", centerScore, lowerPenalty)
	}
	if centerScore <= upperPenalty {
		t.Fatalf("expected center score %.3f to exceed upper penalty %.3f", centerScore, upperPenalty)
	}
	if lowerPenalty <= 0 {
		t.Fatalf("expected lower penalty to remain positive, got %.3f", lowerPenalty)
	}
}

func TestSolutionSnapshot(t *testing.T) {
	solution := &Solution{
		Weights: map[IngredientID]float64{
			HeavyCream.ID:    0.45,
			WholeMilk.ID:     0.35,
			Sugar.ID:         0.15,
			NonfatDryMilk.ID: 0.05,
		},
		Lots: map[IngredientID]LotDescriptor{
			HeavyCream.ID:    HeavyCream.DefaultLot(),
			WholeMilk.ID:     WholeMilk.DefaultLot(),
			Sugar.ID:         Sugar.DefaultLot(),
			NonfatDryMilk.ID: NonfatDryMilk.DefaultLot(),
		},
	}

	snapshot, err := solution.Snapshot(MixOptions{})
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if snapshot.TotalMassKg <= 0 {
		t.Fatalf("expected positive mass, got %.4f", snapshot.TotalMassKg)
	}
	if snapshot.ViscosityAtServe <= 0 {
		t.Fatalf("expected positive viscosity, got %.6f", snapshot.ViscosityAtServe)
	}
}
