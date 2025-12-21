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

func assertCompositionWithinTarget(t *testing.T, target creamery.Composition, achieved creamery.Composition, context string) {
	t.Helper()
	check := func(name string, interval creamery.Interval, got float64) {
		assertIntervalContains(t, interval, got, fmt.Sprintf("%s %s", context, name))
	}
	check("fat", target.Fat, achieved.Fat.Mid())
	check("msnf", target.MSNF, achieved.MSNF.Mid())
	check("sugar", target.Sugar, achieved.Sugar.Mid())
	check("other", target.Other, achieved.Other.Mid())
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
