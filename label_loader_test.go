package creamery

import "testing"

func TestFDALabelLoader(t *testing.T) {
	t.Parallel()

	labels := AllFDALabels()
	if len(labels) == 0 {
		t.Fatal("no labels loaded from labels/ directory")
	}

	// Check that haagen label is loaded
	label, ok := labels[LabelHaagenDazsVanilla]
	if !ok {
		t.Fatalf("label %q missing from loaded definitions", LabelHaagenDazsVanilla)
	}
	if label.Name != "Haagen-Dazs Vanilla" {
		t.Errorf("name = %q, want %q", label.Name, "Haagen-Dazs Vanilla")
	}
	if label.PintMassGrams != 387 {
		t.Errorf("PintMassGrams = %v, want %v", label.PintMassGrams, 387)
	}
	if label.Facts.Calories != 320 {
		t.Errorf("Calories = %v, want %v", label.Facts.Calories, 320)
	}
}

func TestSolveFDALabel(t *testing.T) {
	t.Parallel()

	label, ok := FDALabelByKey(LabelHaagenDazsVanilla)
	if !ok {
		t.Fatalf("label %q missing", LabelHaagenDazsVanilla)
	}
	result, err := SolveFDALabel(label, DefaultIngredientCatalog())
	if err != nil {
		t.Fatalf("solve failed: %v", err)
	}
	if result.Name != "Haagen-Dazs Vanilla" {
		t.Errorf("name = %q, want %q", result.Name, "Haagen-Dazs Vanilla")
	}
	if result.Recipe == nil {
		t.Error("expected non-nil Recipe")
	}
}
