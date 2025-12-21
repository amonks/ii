package creamery

import "testing"

func TestSpecFromCatalogErrorsOnUnknownKey(t *testing.T) {
	if _, err := specFromCatalog("not_a_real_ingredient_key"); err == nil {
		t.Fatalf("expected error when resolving unknown catalog key")
	}
}
