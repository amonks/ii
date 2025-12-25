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

func TestLabelScenarioFromFDA(t *testing.T) {
	t.Parallel()

	def, ok := LabelScenarioByKey(LabelHaagenDazsVanilla)
	if !ok {
		t.Fatalf("label %q missing from scenario definitions", LabelHaagenDazsVanilla)
	}
	if def.Name != "Haagen-Dazs Vanilla" {
		t.Errorf("name = %q, want %q", def.Name, "Haagen-Dazs Vanilla")
	}
	if len(def.Lots) == 0 {
		t.Error("expected non-empty Lots")
	}
	if len(def.Groups) == 0 {
		t.Error("expected non-empty Groups")
	}
}
