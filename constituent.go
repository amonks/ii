package creamery

import "fmt"

// ConstituentSet enumerates the fractional components tracked for each profile.
type ConstituentSet struct {
	Water        float64
	Fat          float64
	Protein      float64
	Lactose      float64
	Sucrose      float64
	Glucose      float64
	Fructose     float64
	Maltodextrin float64
	Polyols      float64
	Ash          float64
	OtherSolids  float64
}

// ConstituentsFromProfile extracts midpoint values for each component.
func ConstituentsFromProfile(profile ConstituentProfile) ConstituentSet {
	return ConstituentSet{
		Water:        profile.Components.Water.Mid(),
		Fat:          profile.Components.Fat.Mid(),
		Protein:      profile.Components.Protein.Mid(),
		Lactose:      profile.Components.Lactose.Mid(),
		Sucrose:      profile.Components.Sucrose.Mid(),
		Glucose:      profile.Components.Glucose.Mid(),
		Fructose:     profile.Components.Fructose.Mid(),
		Maltodextrin: profile.Components.Maltodextrin.Mid(),
		Polyols:      profile.Components.Polyols.Mid(),
		Ash:          profile.Components.Ash.Mid(),
		OtherSolids:  profile.Components.OtherSolids.Mid(),
	}
}

// TotalSolids returns the sum of all non-water components.
func (c ConstituentSet) TotalSolids() float64 {
	return c.Fat + c.Protein + c.Lactose + c.Sucrose + c.Glucose + c.Fructose + c.Maltodextrin + c.Polyols + c.Ash + c.OtherSolids
}

const solidsTolerance = 1e-2

// Validate ensures all constituent intervals fall within physical ranges.
func (c ConstituentComponents) Validate() error {
	checks := []struct {
		name string
		iv   Interval
	}{
		{"water", c.Water},
		{"fat", c.Fat},
		{"msnf", c.MSNF},
		{"protein", c.Protein},
		{"lactose", c.Lactose},
		{"sucrose", c.Sucrose},
		{"glucose", c.Glucose},
		{"fructose", c.Fructose},
		{"maltodextrin", c.Maltodextrin},
		{"polyols", c.Polyols},
		{"ash", c.Ash},
		{"other solids", c.OtherSolids},
	}
	for _, check := range checks {
		if check.iv.Lo < 0 || check.iv.Hi > 1 {
			return fmt.Errorf("constituent %s out of range: %s", check.name, check.iv.String())
		}
		if check.iv.Lo > check.iv.Hi {
			return fmt.Errorf("constituent %s has lo > hi: %s", check.name, check.iv.String())
		}
	}

	totalSolids := CompositionFromComponents(c).TotalSolids()
	if totalSolids.Hi > 1+solidsTolerance {
		return fmt.Errorf("constituent solids exceed 100%% (hi %.4f)", totalSolids.Hi)
	}
	if totalSolids.Lo+c.Water.Lo > 1+solidsTolerance {
		return fmt.Errorf("constituent solids plus water exceed 100%% (lo %.4f)", totalSolids.Lo+c.Water.Lo)
	}
	return nil
}
