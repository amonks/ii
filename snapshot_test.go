package creamery

import (
	"math"
	"testing"
)

func TestBatchSnapshotTotals(t *testing.T) {
	cream := makeSpecFromFractions("Test Cream", ComponentFractions{
		Fat:  Point(0.36),
		MSNF: Point(0.08),
	})
	sugar := makeSpecFromFractions("Test Sugar", ComponentFractions{
		Sucrose: Point(1.0),
	})

	recipe := []RecipeComponent{
		{Ingredient: cream.DefaultLot(), MassKg: 60},
		{Ingredient: sugar.DefaultLot(), MassKg: 40},
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

func TestBatchSnapshotFormulationBreakdownMatchesRecipe(t *testing.T) {
	components := []RecipeComponent{
		{Ingredient: makeSpecFromFractions("Cream", ComponentFractions{
			Fat:  Point(0.36),
			MSNF: Point(0.08),
		}).DefaultLot(), MassKg: 60},
		{Ingredient: makeSpecFromFractions("Sugar", ComponentFractions{
			Sucrose: Point(1),
		}).DefaultLot(), MassKg: 40},
	}
	recipe, err := NewRecipe(components, 0)
	if err != nil {
		t.Fatalf("unexpected recipe error: %v", err)
	}
	snapshot, err := NewBatchSnapshot(components)
	if err != nil {
		t.Fatalf("unexpected snapshot error: %v", err)
	}
	formFromSnapshot, err := snapshot.FormulationBreakdown()
	if err != nil {
		t.Fatalf("snapshot formulation error: %v", err)
	}
	formFromRecipe, err := recipe.Formulation()
	if err != nil {
		t.Fatalf("recipe formulation error: %v", err)
	}

	assertFloatClose(t, formFromRecipe.MilkfatPct, formFromSnapshot.MilkfatPct, "milkfat pct")
	assertFloatClose(t, formFromRecipe.SNFPct, formFromSnapshot.SNFPct, "snf pct")
	assertFloatClose(t, formFromRecipe.WaterPct, formFromSnapshot.WaterPct, "water pct")
	assertFloatClose(t, formFromRecipe.ProteinPct, formFromSnapshot.ProteinPct, "protein pct")
	for key, val := range formFromRecipe.SugarsPct {
		assertFloatClose(t, val, formFromSnapshot.SugarsPct[key], "sugar "+key)
	}
}

func TestBatchSnapshotNutritionFactsSummaryMatchesRecipe(t *testing.T) {
	components := []RecipeComponent{
		{Ingredient: makeSpecFromFractions("Cream", ComponentFractions{
			Fat:  Point(0.36),
			MSNF: Point(0.08),
		}).DefaultLot(), MassKg: 60},
		{Ingredient: makeSpecFromFractions("Sugar", ComponentFractions{
			Sucrose: Point(1),
		}).DefaultLot(), MassKg: 40},
	}
	recipe, err := NewRecipe(components, 0)
	if err != nil {
		t.Fatalf("unexpected recipe error: %v", err)
	}
	snapshot, err := NewBatchSnapshot(components)
	if err != nil {
		t.Fatalf("unexpected snapshot error: %v", err)
	}
	const serving = 150.0
	factsFromRecipe, err := recipe.NutritionFacts(serving, 0)
	if err != nil {
		t.Fatalf("recipe nutrition error: %v", err)
	}
	factsFromSnapshot, err := snapshot.NutritionFactsSummary(serving, 0)
	if err != nil {
		t.Fatalf("snapshot nutrition error: %v", err)
	}

	assertFloatClose(t, factsFromRecipe.Calories, factsFromSnapshot.Calories, "calories")
	assertFloatClose(t, factsFromRecipe.TotalFatGrams, factsFromSnapshot.TotalFatGrams, "fat grams")
	assertFloatClose(t, factsFromRecipe.TotalSugarsGrams, factsFromSnapshot.TotalSugarsGrams, "sugars grams")
	assertFloatClose(t, factsFromRecipe.ProteinGrams, factsFromSnapshot.ProteinGrams, "protein grams")
}

func assertFloatClose(t *testing.T, want, got float64, label string) {
	t.Helper()
	if math.Abs(want-got) > 1e-6 {
		t.Fatalf("%s mismatch: want %.6f got %.6f", label, want, got)
	}
}
