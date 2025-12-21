package creamery

import "testing"

func TestDefaultIngredientCatalogProvidesInstances(t *testing.T) {
	catalog := DefaultIngredientCatalog()
	inst, ok := catalog.InstanceByKey("sucrose")
	if !ok {
		t.Fatalf("default catalog missing sucrose")
	}
	if inst.Definition == nil {
		t.Fatalf("sucrose instance missing definition")
	}
	if got := inst.Definition.Profile.Components.Sucrose.Mid(); got < 0.99 {
		t.Fatalf("expected sucrose fraction near 1, got %.2f", got)
	}
}

func TestStandardSpecsIncludeHeavyCream(t *testing.T) {
	specs := StandardSpecMap()
	hc, ok := specs[HeavyCream.ID]
	if !ok {
		t.Fatalf("heavy cream missing from standard spec map")
	}
	fat := hc.Profile.Components.Fat.Mid()
	if fat < 0.3 {
		t.Fatalf("expected heavy cream fat near 0.36, got %.3f", fat)
	}
}

func TestCustomCatalogRegistersKeys(t *testing.T) {
	spec := SpecFromComposition("My Sugar", PointComposition(0, 0, 1, 0))
	spec.Key = NewIngredientKey("custom_sugar")
	spec = renameSpec(spec, "Fancy Sugar")

	catalog := NewIngredientCatalog([]IngredientDefinition{spec})
	inst, ok := catalog.InstanceByKey("custom_sugar")
	if !ok {
		t.Fatalf("expected catalog to resolve custom key")
	}
	if got := inst.DisplayName(); got != "Fancy Sugar" {
		t.Fatalf("expected renamed spec to propagate, got %s", got)
	}
}
