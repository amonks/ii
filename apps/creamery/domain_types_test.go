package creamery

import "testing"

func TestNewIngredientNormalizes(t *testing.T) {
	profile := ConstituentProfile{
		ID:   "",
		Name: " Heavy Cream ",
		Components: CompositionRange{
			Fat:     Point(0.3),
			MSNF:    Point(0.08),
			Water:   Point(0.5),
			Sucrose: Point(0.12),
		},
	}
	def := NewIngredient(profile, IngredientKey("HEAVY CREAM"))
	if def.ID == "" {
		t.Fatalf("expected normalized ID, got empty")
	}
	if def.Name != "Heavy Cream" {
		t.Fatalf("expected name normalized, got %q", def.Name)
	}
	if def.Key == "" {
		t.Fatalf("expected key normalized, got empty")
	}
	if err := def.Profile.Components.Validate(); err != nil {
		t.Fatalf("profile should validate: %v", err)
	}
}

func TestLotEffectiveProfileOverride(t *testing.T) {
	profile := ConstituentProfile{
		ID:   IngredientID("cream"),
		Name: "Cream",
		Components: CompositionRange{
			Fat:     Point(0.3),
			MSNF:    Point(0.08),
			Water:   Point(0.5),
			Sucrose: Point(0.12),
		},
	}
	def := NewIngredient(profile, "")
	lot := NewLot(&def)

	override := profile
	override.Components.Fat = Point(0.35)
	lot.SetProfileOverride(override)
	effective := lot.EffectiveProfile()
	if effective.Components.Fat.Lo != 0.35 {
		t.Fatalf("expected override fat 0.35, got %.2f", effective.Components.Fat.Lo)
	}
	if effective.ID != def.ID {
		t.Fatalf("expected override to retain definition ID")
	}
}

func TestLotDisplayNameFallback(t *testing.T) {
	profile := ConstituentProfile{
		ID:         IngredientID("test"),
		Name:       "",
		Components: CompositionRange{},
	}
	def := NewIngredient(profile, "")
	lot := Lot{Definition: &def}
	if got := lot.DisplayName(); got == "" {
		t.Fatalf("expected fallback display name, got empty")
	}
}
