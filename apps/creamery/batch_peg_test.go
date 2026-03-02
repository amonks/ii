package creamery

import (
	"testing"
	"time"
)

func TestParseBatch(t *testing.T) {
	content := `Date: 2025-12-14

Ingredients:
  432g cream36
  266.9g whole_milk
  111.5g skim_milk_powder

Process Notes:
  circulator 69c 30m
  homogenize 45s

Tasting Notes:
  tastes good
`

	entry, err := ParseBatch(content)
	if err != nil {
		t.Fatalf("ParseBatch failed: %v", err)
	}

	expectedDate := time.Date(2025, 12, 14, 0, 0, 0, 0, time.UTC)
	if !entry.Date.Equal(expectedDate) {
		t.Errorf("Date = %v, want %v", entry.Date, expectedDate)
	}

	if len(entry.Ingredients) != 3 {
		t.Errorf("len(Ingredients) = %d, want 3", len(entry.Ingredients))
	}

	if entry.Ingredients[0].Key != "cream36" {
		t.Errorf("Ingredients[0].Key = %q, want %q", entry.Ingredients[0].Key, "cream36")
	}

	if len(entry.ProcessNotes) != 2 {
		t.Errorf("len(ProcessNotes) = %d, want 2", len(entry.ProcessNotes))
	}

	if len(entry.TastingNotes) != 1 {
		t.Errorf("len(TastingNotes) = %d, want 1", len(entry.TastingNotes))
	}
}

func TestLoadBatchesFromDir(t *testing.T) {
	batches, err := LoadBatchesFromDir("batches")
	if err != nil {
		t.Fatalf("LoadBatchesFromDir failed: %v", err)
	}

	if len(batches) != 5 {
		t.Errorf("len(batches) = %d, want 5", len(batches))
	}

	// Check that batches are sorted by date (newest first for display)
	for i := 1; i < len(batches); i++ {
		if batches[i].Date.After(batches[i-1].Date) {
			t.Errorf("batches not sorted newest-first: %v after %v", batches[i].Date, batches[i-1].Date)
		}
	}

	// Check sequence numbers (1 = oldest batch, N = newest)
	n := len(batches)
	for i, batch := range batches {
		want := n - i
		if batch.Sequence != want {
			t.Errorf("batch[%d].Sequence = %d, want %d", i, batch.Sequence, want)
		}
	}
}
