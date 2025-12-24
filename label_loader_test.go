package creamery

import "testing"

func TestLabelLoaderSpecOverrides(t *testing.T) {
	t.Parallel()

	defs, err := loadLabelDefinitionsFromFile("labels.json")
	if err != nil {
		t.Fatalf("loadLabelDefinitionsFromFile error: %v", err)
	}
	def, ok := defs[LabelJenisSweetCream]
	if !ok {
		t.Fatalf("label %q missing from loaded definitions", LabelJenisSweetCream)
	}
	if len(def.IngredientSpecs) == 0 {
		t.Fatalf("label %q missing IngredientSpecs overrides", LabelJenisSweetCream)
	}
	found := false
	for _, spec := range def.IngredientSpecs {
		if spec.Name == "Nonfat Milk" {
			found = true
			if spec.ID != NonfatMilkVariable.ID {
				t.Fatalf("Nonfat milk spec ID = %s, want %s", spec.ID, NonfatMilkVariable.ID)
			}
			got := spec.Profile.Components.MSNF
			want := NonfatMilkVariable.Profile.Components.MSNF
			if got.Lo != want.Lo || got.Hi != want.Hi {
				t.Fatalf("nonfat milk MSNF range = %v, want %v", got, want)
			}
			break
		}
	}
	if !found {
		t.Fatalf("nonfat milk spec override missing for %q", LabelJenisSweetCream)
	}
	if len(def.ScenarioSpecs) == 0 {
		t.Fatalf("label %q missing ScenarioSpecs", LabelJenisSweetCream)
	}
}
