package creamery

import "testing"

func TestBatchComponentsAndFractions(t *testing.T) {
	cream := makeSpecFromFractions("cream", ComponentFractions{Fat: Point(0.36)})
	milk := makeSpecFromFractions("milk", ComponentFractions{Fat: Point(0.03)})

	blend := Blend{Components: []Portion{
		{Lot: cream.DefaultLot(), Fraction: 0.4},
		{Lot: milk.DefaultLot(), Fraction: 0.6},
	}}

	batch := NewBatch(blend, 50)
	components := batch.Components()
	if len(components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(components))
	}

	if got := components[0].MassKg; got < 19.999 || got > 20.001 {
		t.Fatalf("expected first component to be ~20kg, got %.3fkg", got)
	}
	if got := components[1].MassKg; got < 29.999 || got > 30.001 {
		t.Fatalf("expected second component to be ~30kg, got %.3fkg", got)
	}

	fractions := batch.FractionsByName()
	if len(fractions) != 2 {
		t.Fatalf("expected 2 fraction entries, got %d", len(fractions))
	}
	if frac := fractions["cream"]; frac < 0.399 || frac > 0.401 {
		t.Fatalf("unexpected cream fraction %.3f", frac)
	}
	if frac := fractions["milk"]; frac < 0.599 || frac > 0.601 {
		t.Fatalf("unexpected milk fraction %.3f", frac)
	}

	snapshot, err := batch.Snapshot()
	if err != nil {
		t.Fatalf("batch snapshot returned error: %v", err)
	}
	if snapshot.TotalMassKg != 50 {
		t.Fatalf("expected snapshot mass 50kg, got %.2fkg", snapshot.TotalMassKg)
	}
}
