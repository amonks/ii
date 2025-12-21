package creamery

import (
	"math"
	"testing"
)

func TestBatchSnapshotTotals(t *testing.T) {
	cream := SpecFromComposition("Test Cream", Composition{
		Fat:   Point(0.36),
		MSNF:  Point(0.08),
		Sugar: Point(0),
		Other: Point(0),
	})
	sugar := SpecFromComposition("Test Sugar", Composition{
		Sugar: Point(1.0),
	})

	recipe := []RecipeComponent{
		{Ingredient: NewIngredientInstance(cream), MassKg: 60},
		{Ingredient: NewIngredientInstance(sugar), MassKg: 40},
	}

	snapshot, err := NewBatchSnapshot(recipe)
	if err != nil {
		t.Fatalf("unexpected error building snapshot: %v", err)
	}
	if snapshot.TotalMassKg != 100 {
		t.Fatalf("expected total mass 100kg, got %.2f", snapshot.TotalMassKg)
	}
	expectedFat := 60 * 0.36
	if math.Abs(snapshot.FatMassKg-expectedFat) > 1e-6 {
		t.Fatalf("expected fat mass %.2f, got %.4f", expectedFat, snapshot.FatMassKg)
	}
	expectedSugar := 40 * 1.0
	if math.Abs(snapshot.AddedSugars.Mid-expectedSugar) > 1e-6 {
		t.Fatalf("expected sugar mass %.2f, got %.4f", expectedSugar, snapshot.AddedSugars.Mid)
	}
}
