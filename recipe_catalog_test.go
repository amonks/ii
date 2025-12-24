package creamery

import "testing"

func TestCuratedRecipesLoadWithoutIssues(t *testing.T) {
	catalog := DefaultIngredientCatalog()
	rc, err := LoadRecipeCatalogFromFiles([]string{"recipes"}, catalog)
	if err != nil {
		t.Fatalf("LoadRecipeCatalogFromFiles returned error: %v", err)
	}
	if len(rc.Entries) == 0 {
		t.Fatalf("expected curated recipes to load, got zero entries")
	}
	for _, entry := range rc.Entries {
		if len(entry.Issues) > 0 {
			t.Fatalf("entry %q has issues: %v", entry.Label, entry.Issues)
		}
	}
}
