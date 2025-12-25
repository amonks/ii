package creamery

import (
	"os"
	"testing"
)

func TestParseLabel_WithSimpleIngredients(t *testing.T) {
	content, err := os.ReadFile("testdata/label_v5.fda")
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}

	label, err := ParseLabel(string(content))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if label.ID != "test" {
		t.Errorf("got ID %q, want %q", label.ID, "test")
	}
	if label.Name != "Test Product" {
		t.Errorf("got Name %q, want %q", label.Name, "Test Product")
	}
	if label.PintMassGrams != 387 {
		t.Errorf("got PintMassGrams %v, want %v", label.PintMassGrams, 387)
	}
	if label.Facts.Calories != 320 {
		t.Errorf("got Calories %v, want %v", label.Facts.Calories, 320)
	}

	wantIngredients := []string{"skim_milk", "cane_sugar", "egg_yolk"}
	if len(label.Ingredients) != len(wantIngredients) {
		t.Fatalf("got %d ingredients, want %d", len(label.Ingredients), len(wantIngredients))
	}
	for i, want := range wantIngredients {
		if label.Ingredients[i].ID != want {
			t.Errorf("ingredient %d: got ID %q, want %q", i, label.Ingredients[i].ID, want)
		}
	}
}
