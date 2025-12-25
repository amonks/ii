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

func TestParseLabel_WithCompoundIngredients(t *testing.T) {
	content, err := os.ReadFile("testdata/label_v6.fda")
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

	// Check all ingredients are flattened correctly
	wantIngredients := []string{"cream_fat", "cream_serum", "skim_milk", "liquid_sugar_sucrose", "liquid_sugar_water", "egg_yolk"}
	if len(label.Ingredients) != len(wantIngredients) {
		t.Fatalf("got %d ingredients, want %d: %v", len(label.Ingredients), len(wantIngredients), label.Ingredients)
	}
	for i, want := range wantIngredients {
		if label.Ingredients[i].ID != want {
			t.Errorf("ingredient %d: got ID %q, want %q", i, label.Ingredients[i].ID, want)
		}
	}

	// Check groups
	if len(label.Groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(label.Groups))
	}

	// Check Cream group
	cream := label.Groups[0]
	if cream.Name != "Cream" {
		t.Errorf("group 0: got name %q, want %q", cream.Name, "Cream")
	}
	if len(cream.Members) != 2 || cream.Members[0] != "cream_fat" || cream.Members[1] != "cream_serum" {
		t.Errorf("group 0: got members %v, want [cream_fat cream_serum]", cream.Members)
	}
	if cream.FractionBounds == nil {
		t.Fatal("group 0: FractionBounds is nil")
	}
	bound, ok := cream.FractionBounds["cream_fat"]
	if !ok {
		t.Fatal("group 0: missing fraction bound for cream_fat")
	}
	if bound.Lo != 0.18 || bound.Hi != 0.5 {
		t.Errorf("group 0: got cream_fat bounds %v, want {0.18 0.5}", bound)
	}

	// Check Liquid Sugar group
	liquidSugar := label.Groups[1]
	if liquidSugar.Name != "Liquid Sugar" {
		t.Errorf("group 1: got name %q, want %q", liquidSugar.Name, "Liquid Sugar")
	}
	if !liquidSugar.EnforceOrder {
		t.Error("group 1: expected EnforceOrder to be true")
	}
}
