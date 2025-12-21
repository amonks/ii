package creamery

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
