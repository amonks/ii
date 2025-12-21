package creamery

import (
	"strings"
	"testing"
)

func TestParseBatchLog(t *testing.T) {
	raw := `
date: 2025-01-05
recipe: vanilla_base_v3
ingredients:
  - 3.6 kg heavy_cream
  - 2.8 kg whole_milk
  - 2100 g sucrose
process_notes:
  Pasteurized to 85C.
  circulator 69c 30m
    not great; core temp too low
tasting_notes:
  Dense mouthfeel.

%%
date: 2025-01-10
ingredients:
  - 3.0kg heavy_cream
  - 1.2 kg sucrose
process_notes:
  Long infusion
`

	entries, err := ParseBatchLog(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseBatchLog returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	first := entries[0]
	if first.Date.IsZero() {
		t.Fatalf("expected date to be parsed")
	}
	if len(first.Ingredients) != 3 {
		t.Fatalf("expected 3 ingredients, got %d", len(first.Ingredients))
	}
	if mass := first.Ingredients[2].MassKg; mass < 2.099 || mass > 2.101 {
		t.Fatalf("expected sucrose mass 2.1 kg, got %.3f", mass)
	}
	totalMass := 0.0
	for _, ing := range first.Ingredients {
		totalMass += ing.MassKg
	}
	if totalMass < 8.49 || totalMass > 8.51 {
		t.Fatalf("expected total mass 8.5 kg, got %.3f", totalMass)
	}

	second := entries[1]
	lines := strings.Split(first.ProcessNotes[0], "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[2], "  ") {
		t.Fatalf("expected multiline process note with retained indent, got %q", first.ProcessNotes[0])
	}
	if len(second.ProcessNotes) != 1 {
		t.Fatalf("expected one process note, got %d", len(second.ProcessNotes))
	}
	if len(second.Ingredients) != 2 {
		t.Fatalf("expected 2 ingredients, got %d", len(second.Ingredients))
	}
}
