package creamery

import (
	"os"
	"testing"
)

func TestParseLabel_WithNutritionFacts(t *testing.T) {
	content, err := os.ReadFile("testdata/label_v4.fda")
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
	if label.Facts.ServingSizeGrams != 129 {
		t.Errorf("got ServingSizeGrams %v, want %v", label.Facts.ServingSizeGrams, 129)
	}
	if label.Facts.Calories != 320 {
		t.Errorf("got Calories %v, want %v", label.Facts.Calories, 320)
	}
	if label.Facts.TotalFatGrams != 21 {
		t.Errorf("got TotalFatGrams %v, want %v", label.Facts.TotalFatGrams, 21)
	}
	if label.Facts.SaturatedFatGrams != 13 {
		t.Errorf("got SaturatedFatGrams %v, want %v", label.Facts.SaturatedFatGrams, 13)
	}
	if label.Facts.TransFatGrams != 1 {
		t.Errorf("got TransFatGrams %v, want %v", label.Facts.TransFatGrams, 1)
	}
	if label.Facts.CholesterolMg != 95 {
		t.Errorf("got CholesterolMg %v, want %v", label.Facts.CholesterolMg, 95)
	}
	if label.Facts.TotalCarbGrams != 26 {
		t.Errorf("got TotalCarbGrams %v, want %v", label.Facts.TotalCarbGrams, 26)
	}
	if label.Facts.TotalSugarsGrams != 25 {
		t.Errorf("got TotalSugarsGrams %v, want %v", label.Facts.TotalSugarsGrams, 25)
	}
	if label.Facts.AddedSugarsGrams != 18 {
		t.Errorf("got AddedSugarsGrams %v, want %v", label.Facts.AddedSugarsGrams, 18)
	}
	if label.Facts.ProteinGrams != 6 {
		t.Errorf("got ProteinGrams %v, want %v", label.Facts.ProteinGrams, 6)
	}
	if label.Facts.SodiumMg != 75 {
		t.Errorf("got SodiumMg %v, want %v", label.Facts.SodiumMg, 75)
	}
}
