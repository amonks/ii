package creamery

import (
	"strings"
	"testing"
)

func TestAnalyzeBatchLog(t *testing.T) {
	data := `
date: 2025-02-01
ingredients:
  - 4 kg heavy_cream
  - 1 kg sucrose
  - 0.5 kg corn_syrup_42

%%
date: 2025-02-02
ingredients:
  - 3 kg heavy_cream
  - 2 kg whole_milk
  - 1.2 kg sucrose
`

	entries, err := ParseBatchLog(strings.NewReader(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	catalog := DefaultIngredientCatalog()
	analytics := AnalyzeBatchLog(entries, catalog)

	if analytics.Summary.TotalBatches != 2 {
		t.Fatalf("expected 2 batches, got %d", analytics.Summary.TotalBatches)
	}
	if analytics.Summary.ValidSnapshots != 2 {
		t.Fatalf("expected 2 valid snapshots, got %d", analytics.Summary.ValidSnapshots)
	}
	if analytics.Summary.EarliestDate.IsZero() || analytics.Summary.LatestDate.IsZero() {
		t.Fatalf("expected date range to be populated")
	}
	if len(analytics.Summary.IngredientTotals) == 0 {
		t.Fatalf("expected ingredient totals")
	}
	var found bool
	for _, usage := range analytics.Summary.IngredientTotals {
		if usage.Key == NewIngredientKey("heavy_cream") {
			if usage.TotalMassKg < 6.99 || usage.TotalMassKg > 7.01 {
				t.Fatalf("expected ~7kg heavy cream, got %.3f", usage.TotalMassKg)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected heavy cream usage to be present")
	}
}
