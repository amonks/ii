package creamery_test

import (
	"fmt"
	"testing"

	"github.com/amonks/creamery"
)

const intervalEpsilon = 1e-4

func assertIntervalContains(t *testing.T, interval creamery.Interval, value float64, label string) {
	t.Helper()
	if interval.Lo == 0 && interval.Hi == 0 {
		return
	}
	if value < interval.Lo-intervalEpsilon || value > interval.Hi+intervalEpsilon {
		t.Fatalf("%s = %.4f outside interval %s", label, value, interval.StringAbs())
	}
}

func assertFractionsWithinTarget(t *testing.T, target creamery.ComponentFractions, achieved creamery.ComponentFractions, context string) {
	t.Helper()
	check := func(name string, interval creamery.Interval, got float64) {
		assertIntervalContains(t, interval, got, fmt.Sprintf("%s %s", context, name))
	}
	check("fat", target.Fat, achieved.Fat.Mid())
	check("msnf", target.EffectiveMSNF(), achieved.EffectiveMSNF().Mid())
	check("added sugar", target.AddedSugarsInterval(), achieved.AddedSugarsInterval().Mid())
	check("other solids", target.OtherSolids, achieved.OtherSolids.Mid())
}

func assertSweetenersMatchTarget(t *testing.T, target creamery.FormulationTarget, sweet creamery.SweetenerAnalysis, context string) {
	t.Helper()
	if target.HasPOD() {
		assertIntervalContains(t, target.POD, sweet.TotalPOD, context+" POD")
	}
	if target.HasPAC() {
		assertIntervalContains(t, target.PAC, sweet.TotalPAC, context+" PAC")
	}
}

func newSpec(t *testing.T, name string, build func(*creamery.ComponentFractions)) creamery.Ingredient {
	t.Helper()
	comps := creamery.ComponentFractions{}
	if build != nil {
		build(&comps)
	}
	comps = creamery.EnsureWater(comps)
	profile := creamery.ConstituentProfile{
		ID:         creamery.NewIngredientID(name),
		Name:       name,
		Components: comps,
	}
	return creamery.SpecFromProfile(profile)
}
