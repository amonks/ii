package creamery

import "math"

// FDA nutrition label rounding rules.
// These define the intervals that a displayed value could represent.

// FatInterval returns the interval of actual fat content (in grams)
// that would be displayed as the given value on an FDA label.
//
// FDA rounding rules for fat:
//   - <0.5g: may be expressed as 0
//   - 0.5g to <5g: nearest 0.5g
//   - >=5g: nearest 1g
func FatInterval(displayed float64) Interval {
	if displayed == 0 {
		return Range(0, 0.5)
	}
	if displayed < 5 {
		// Rounded to nearest 0.5g
		return Range(displayed-0.25, displayed+0.25)
	}
	// Rounded to nearest 1g
	return Range(displayed-0.5, displayed+0.5)
}

// ProteinInterval returns the interval of actual protein content
// that would be displayed as the given value.
//
// FDA rounding: nearest 1g (or 0 if <1g)
func ProteinInterval(displayed float64) Interval {
	if displayed == 0 {
		return Range(0, 0.5)
	}
	return Range(displayed-0.5, displayed+0.5)
}

// CarbInterval returns the interval of actual carbohydrate content
// that would be displayed as the given value.
//
// FDA rounding: nearest 1g (or 0 if <1g)
func CarbInterval(displayed float64) Interval {
	if displayed == 0 {
		return Range(0, 0.5)
	}
	return Range(displayed-0.5, displayed+0.5)
}

// SugarInterval returns the interval of actual sugar content
// that would be displayed as the given value.
//
// FDA rounding: nearest 1g (or "less than 1g" if <1g)
func SugarInterval(displayed float64) Interval {
	if displayed == 0 {
		return Range(0, 0.5)
	}
	return Range(displayed-0.5, displayed+0.5)
}

// CalorieInterval returns the interval of actual calorie content
// that would be displayed as the given value.
//
// FDA rounding rules for calories:
//   - <5 cal: may be expressed as 0
//   - 5-50 cal: nearest 5
//   - >50 cal: nearest 10
func CalorieInterval(displayed float64) Interval {
	if displayed == 0 {
		return Range(0, 5)
	}
	if displayed <= 50 {
		return Range(displayed-2.5, displayed+2.5)
	}
	return Range(displayed-5, displayed+5)
}

// NutritionLabel represents an FDA nutrition facts label.
type NutritionLabel struct {
	ServingSize float64 // grams
	Calories    float64
	TotalFat    float64 // grams
	Protein     float64 // grams
	TotalCarbs  float64 // grams
	Sugars      float64 // grams (total sugars: lactose + intrinsic + added)
	AddedSugars float64 // grams of added sugars (0 if not declared or none)
}

// ToTarget converts FDA label values to a target Composition with
// appropriate uncertainty intervals from rounding.
func (l NutritionLabel) ToTarget() Composition {
	serving := l.ServingSize

	// Convert gram intervals to fraction intervals
	fatGrams := FatInterval(l.TotalFat)
	proteinGrams := ProteinInterval(l.Protein)
	carbGrams := CarbInterval(l.TotalCarbs)
	sugarGrams := SugarInterval(l.Sugars)
	addedSugarGrams := sugarGrams
	if l.AddedSugars > 0 {
		addedSugarGrams = SugarInterval(l.AddedSugars)
	}

	// Fat fraction
	fatFrac := Interval{
		Lo: math.Max(0, fatGrams.Lo/serving),
		Hi: math.Min(1, fatGrams.Hi/serving),
	}

	// For MSNF: estimate from protein
	// Protein is roughly 36-40% of MSNF (casein + whey), varies by source
	// Use a wide range to be conservative and avoid overconstraining:
	//   Low MSNF: assume protein is high fraction of MSNF (40%)
	//   High MSNF: assume protein is low fraction of MSNF (34%)
	// This gives room for measurement/compositional uncertainty
	msnfFrac := Interval{
		Lo: math.Max(0, (proteinGrams.Lo/0.40)/serving),
		Hi: math.Min(1, (proteinGrams.Hi/0.34)/serving),
	}

	// Cap MSNF by available carbs once added sugars are accounted for.
	// Lactose (from MSNF) + fiber/starch must fit in (carbs - added sugar).
	if l.TotalCarbs > 0 {
		// Use the most permissive gap between total carbs and added sugars (hi - lo)
		// so rounding uncertainty doesn't make the cap too aggressive.
		maxLactoseGrams := math.Max(0, carbGrams.Hi-addedSugarGrams.Lo)
		if maxLactoseGrams > 0 && serving > 0 {
			maxMSNFByCarbs := math.Min(1, (maxLactoseGrams/serving)/LactoseFractionOfMSNF)
			if maxMSNFByCarbs < msnfFrac.Hi {
				if maxMSNFByCarbs < msnfFrac.Lo {
					msnfFrac.Hi = msnfFrac.Lo
				} else {
					msnfFrac.Hi = maxMSNFByCarbs
				}
			}
		}
	}

	// Sugar fraction
	sugarFrac := Interval{
		Lo: math.Max(0, addedSugarGrams.Lo/serving),
		Hi: math.Min(1, addedSugarGrams.Hi/serving),
	}

	// Other: hard to estimate from label, use wide range
	// Could be 0-5% typically
	otherFrac := Range(0, 0.05)

	return Composition{
		Fat:   fatFrac,
		MSNF:  msnfFrac,
		Sugar: sugarFrac,
		Other: otherFrac,
	}
}

// CaloriesFromComposition estimates calories from a composition.
// Fat: 9 cal/g, Protein: 4 cal/g, Carbs: 4 cal/g
func CaloriesFromComposition(c Composition, servingGrams float64) Interval {
	// Fat contributes 9 cal/g
	fatCal := c.Fat.Scale(servingGrams * 9)

	// MSNF is about 38% protein, 54% lactose (carb), 8% minerals
	// So MSNF contributes: 0.38*4 + 0.54*4 ≈ 3.7 cal/g
	msnfCal := c.MSNF.Scale(servingGrams * 3.7)

	// Sugar contributes 4 cal/g
	sugarCal := c.Sugar.Scale(servingGrams * 4)

	// Other varies (cocoa ~2 cal/g, stabilizers ~0)
	// Use 2 cal/g as rough estimate
	otherCal := c.Other.Scale(servingGrams * 2)

	return fatCal.Add(msnfCal).Add(sugarCal).Add(otherCal)
}
