package creamery

import "fmt"

// ComponentFractions is the canonical percentage-based representation for
// ingredients, targets, and solutions. It aliases ConstituentComponents for
// clarity.
type ComponentFractions = ConstituentComponents

// TotalSolidsInterval returns the sum of all non-water component intervals.
func TotalSolidsInterval(f ComponentFractions) Interval {
	total := Interval{}
	total = total.Add(f.Fat)
	total = total.Add(f.EffectiveMSNF())
	total = total.Add(f.AddedSugarsInterval())
	total = total.Add(f.OtherSolids)
	return total
}

// DerivedWaterInterval computes the implied water interval assuming the listed
// components sum to at most 100% of the mix.
func DerivedWaterInterval(f ComponentFractions) Interval {
	solids := TotalSolidsInterval(f)
	lo := clamp01(1 - solids.Hi)
	hi := clamp01(1 - solids.Lo)
	if lo > hi {
		lo = hi
	}
	return Interval{Lo: lo, Hi: hi}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// EnsureWater populates the water interval if it is unset by deriving the
// remainder fraction.
func EnsureWater(f ComponentFractions) ComponentFractions {
	copy := f
	if copy.Water.Lo == 0 && copy.Water.Hi == 0 {
		copy.Water = DerivedWaterInterval(copy)
	}
	return copy
}

func intervalSpecified(iv Interval) bool {
	return iv.Lo != 0 || iv.Hi != 0
}

func populateMSNFComponents(f ComponentFractions) ComponentFractions {
	copy := f
	if intervalSpecified(copy.MSNF) {
		if !intervalSpecified(copy.Protein) {
			copy.Protein = copy.MSNF.Scale(proteinFractionOfMSNF)
		}
		if !intervalSpecified(copy.Lactose) {
			copy.Lactose = copy.MSNF.Scale(LactoseFractionOfMSNF)
		}
		if !intervalSpecified(copy.Ash) {
			copy.Ash = clampInterval(copy.MSNF.Sub(copy.Protein.Add(copy.Lactose)), 0)
		}
	}
	return copy
}

// ComponentSummary renders a lightweight human-readable description of key
// fractions for logging/debugging.
func ComponentSummary(f ComponentFractions) string {
	return fmt.Sprintf(
		"fat=%s protein=%s lactose=%s added=%s water=%s",
		f.Fat.String(),
		f.Protein.String(),
		f.Lactose.String(),
		f.AddedSugarsInterval().String(),
		f.Water.String(),
	)
}
