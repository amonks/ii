package creamery

import "testing"

func TestConstituentComponentsValidateRejectsOverfull(t *testing.T) {
	comps := ConstituentComponents{
		Water:   Point(0.1),
		Fat:     Point(0.6),
		Sucrose: Point(0.4),
	}
	if err := comps.Validate(); err == nil {
		t.Fatalf("expected solids > 100%% to fail validation")
	}
}

func TestConstituentComponentsValidateAllowsFeasibleMix(t *testing.T) {
	comps := ConstituentComponents{
		Water:   Point(0.6),
		Fat:     Point(0.3),
		Protein: Point(0.04),
		Lactose: Point(0.03),
		Sucrose: Point(0.02),
	}
	if err := comps.Validate(); err != nil {
		t.Fatalf("expected feasible mix to pass validation, got %v", err)
	}
}
