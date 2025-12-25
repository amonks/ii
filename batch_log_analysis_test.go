package creamery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeBatchLog(t *testing.T) {
	// Create temp directory with batch files
	dir := t.TempDir()

	batch1 := `Date: 2025-02-01

Ingredients:
  4kg heavy_cream
  1kg sucrose
  0.5kg corn_syrup_42
`
	batch2 := `Date: 2025-02-02

Ingredients:
  3kg heavy_cream
  2kg whole_milk
  1.2kg sucrose
`

	if err := os.WriteFile(filepath.Join(dir, "2025-02-01.batch"), []byte(batch1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2025-02-02.batch"), []byte(batch2), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := LoadBatchesFromDir(dir)
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
