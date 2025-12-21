package creamery

import "strings"

// IngredientID uniquely identifies an ingredient spec/profile across the system.
type IngredientID string

// String returns the string form of the ID.
func (id IngredientID) String() string {
	return string(id)
}

// NewIngredientID normalizes an arbitrary name into a stable ID (lowercase, snake_case).
func NewIngredientID(name string) IngredientID {
	cleaned := strings.ToLower(strings.TrimSpace(name))
	if cleaned == "" {
		return IngredientID("ingredient")
	}
	var b strings.Builder
	lastSeparator := true
	for _, r := range cleaned {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastSeparator = false
			continue
		}
		if !lastSeparator {
			b.WriteByte('_')
			lastSeparator = true
		}
	}
	id := strings.Trim(b.String(), "_")
	if id == "" {
		return IngredientID("ingredient")
	}
	return IngredientID(id)
}

// ConstituentProfile captures the detailed breakdown (with uncertainty) plus
// functional metadata for an ingredient specification or batch.
type ConstituentProfile struct {
	ID          IngredientID
	Name        string
	Components  ConstituentComponents
	Nutrition   ConstituentNutrition
	Functionals ConstituentFunctionals
	Economics   ConstituentEconomics
}

// ConstituentComponents tracks fractions (per unit mass) for solids and water.
type ConstituentComponents struct {
	Water        Interval
	Fat          Interval
	MSNF         Interval
	Protein      Interval
	Lactose      Interval
	Sucrose      Interval
	Glucose      Interval
	Fructose     Interval
	Maltodextrin Interval
	Polyols      Interval
	Ash          Interval
	OtherSolids  Interval
}

// ConstituentNutrition stores macro-/micro-nutrient data that is not part of the
// basic solids set (e.g., added sugars disclosures, cholesterol).
type ConstituentNutrition struct {
	TransFat           Interval
	SaturatedFat       Interval
	CholesterolMgPerKg Interval
	AddedSugars        Interval
}

// ConstituentFunctionals captures process-specific metadata.
type ConstituentFunctionals struct {
	WaterBinding    Interval
	EmulsifierPower Interval
	Hydrocolloid    bool
	OsmoticCoeff    float64
	VHFactor        float64
	EffectiveMW     float64
	MaltodextrinDP  float64
	PolyolMW        float64
}

// ConstituentEconomics stores cost and related economic metadata.
type ConstituentEconomics struct {
	Cost Interval
}

func pointInterval(v float64) Interval {
	return Point(v)
}

func intervalFromBounds(mid, min, max float64) Interval {
	lo := mid
	hi := mid
	if min > 0 && (lo == 0 || min < lo) {
		lo = min
	}
	if max > 0 && (hi == 0 || max > hi) {
		hi = max
	}
	if lo > hi {
		lo = hi
	}
	return Interval{Lo: lo, Hi: hi}
}

func clampInterval(i Interval, min float64) Interval {
	if i.Lo < min {
		i.Lo = min
	}
	if i.Hi < min {
		i.Hi = min
	}
	if i.Hi < i.Lo {
		i.Hi = i.Lo
	}
	return i
}

// ToProfile converts a ingredientBatch into a ConstituentProfile retaining
// all fractional and functional metadata.
func (d ingredientBatch) ToProfile() ConstituentProfile {
	id := d.ID
	if id == "" {
		id = NewIngredientID(d.Name)
	}
	lactoseLo := d.LactoseMin
	if lactoseLo == 0 {
		lactoseLo = d.Lactose
	}
	lactoseHi := d.LactoseMax
	if lactoseHi == 0 {
		lactoseHi = d.Lactose
	}
	msnfLo := d.Protein + lactoseLo + d.Ash
	msnfHi := d.Protein + lactoseHi + d.Ash
	if msnfHi < msnfLo {
		msnfHi = msnfLo
	}
	profile := ConstituentProfile{
		ID:   id,
		Name: d.Name,
		Components: ConstituentComponents{
			Water:        pointInterval(d.Water),
			Fat:          pointInterval(d.Fat),
			MSNF:         Interval{Lo: msnfLo, Hi: msnfHi},
			Protein:      pointInterval(d.Protein),
			Lactose:      intervalFromBounds(d.Lactose, d.LactoseMin, d.LactoseMax),
			Sucrose:      pointInterval(d.Sucrose),
			Glucose:      pointInterval(d.Glucose),
			Fructose:     pointInterval(d.Fructose),
			Maltodextrin: pointInterval(d.Maltodextrin),
			Polyols:      pointInterval(d.Polyols),
			Ash:          pointInterval(d.Ash),
			OtherSolids:  pointInterval(d.OtherSolids),
		},
		Nutrition: ConstituentNutrition{
			TransFat:           pointInterval(d.TransFat),
			SaturatedFat:       intervalFromBounds(d.SaturatedFat, d.SaturatedFatMin, d.SaturatedFatMax),
			CholesterolMgPerKg: pointInterval(d.CholesterolMgPerKg),
			AddedSugars:        intervalFromBounds(d.AddedSugars, d.AddedSugarsMin, d.AddedSugarsMax),
		},
		Functionals: ConstituentFunctionals{
			WaterBinding:    pointInterval(d.WaterBinding),
			EmulsifierPower: pointInterval(d.EmulsifierPower),
			Hydrocolloid:    d.Hydrocolloid,
			OsmoticCoeff:    d.OsmoticCoeff,
			VHFactor:        d.VHFactor,
			EffectiveMW:     d.EffectiveMW,
			MaltodextrinDP:  d.MaltodextrinDP,
			PolyolMW:        d.PolyolMW,
		},
		Economics: ConstituentEconomics{
			Cost: pointInterval(d.Cost),
		},
	}
	return profile
}

// ProfileFromComposition approximates a constituent profile using the higher-
// level Composition intervals, distributing MSNF into protein/lactose/minerals.
func ProfileFromComposition(id IngredientID, name string, comp Composition) ConstituentProfile {
	if id == "" {
		id = NewIngredientID(name)
	}
	protein := comp.MSNF.Scale(proteinFractionOfMSNF)
	lactose := comp.MSNF.Scale(LactoseFractionOfMSNF)
	minerals := clampInterval(comp.MSNF.Sub(protein.Add(lactose)), 0)
	components := ConstituentComponents{
		Water:        comp.Water(),
		Fat:          comp.Fat,
		MSNF:         comp.MSNF,
		Protein:      protein,
		Lactose:      lactose,
		Ash:          minerals,
		Sucrose:      comp.Sugar,
		OtherSolids:  comp.Other,
		Glucose:      Point(0),
		Fructose:     Point(0),
		Maltodextrin: Point(0),
		Polyols:      Point(0),
	}
	nutrition := ConstituentNutrition{
		TransFat:           Point(0),
		SaturatedFat:       Point(0),
		CholesterolMgPerKg: Point(0),
		AddedSugars:        comp.Sugar,
	}
	functionals := ConstituentFunctionals{
		WaterBinding:    Point(0),
		EmulsifierPower: Point(0),
		Hydrocolloid:    false,
		OsmoticCoeff:    1,
		VHFactor:        1,
		EffectiveMW:     0,
		MaltodextrinDP:  0,
		PolyolMW:        0,
	}
	economics := ConstituentEconomics{Cost: Point(0)}
	return ConstituentProfile{
		ID:          id,
		Name:        name,
		Components:  components,
		Nutrition:   nutrition,
		Functionals: functionals,
		Economics:   economics,
	}
}

// AddedSugarsInterval returns the summed interval for non-lactose sugars.
func (c ConstituentComponents) AddedSugarsInterval() Interval {
	return c.Sucrose.
		Add(c.Glucose).
		Add(c.Fructose).
		Add(c.Maltodextrin).
		Add(c.Polyols)
}

// EffectiveMSNF returns the MSNF interval or derives it from components when missing.
func (c ConstituentComponents) EffectiveMSNF() Interval {
	if c.MSNF.Lo != 0 || c.MSNF.Hi != 0 {
		return c.MSNF
	}
	return c.Protein.Add(c.Lactose).Add(c.Ash)
}

// AddedSugarsInterval exposes the summed added sugars interval on the profile.
func (p ConstituentProfile) AddedSugarsInterval() Interval {
	return p.Components.AddedSugarsInterval()
}

// LactoseInterval returns the lactose interval for the profile.
func (p ConstituentProfile) LactoseInterval() Interval {
	return p.Components.Lactose
}

// ProteinInterval returns the protein interval for the profile.
func (p ConstituentProfile) ProteinInterval() Interval {
	return p.Components.Protein
}

// MSNFInterval returns the milk solids non-fat interval for the profile.
func (p ConstituentProfile) MSNFInterval() Interval {
	return p.Components.EffectiveMSNF()
}

// WaterInterval returns the water interval for the profile.
func (p ConstituentProfile) WaterInterval() Interval {
	return p.Components.Water
}

// OtherSolidsInterval returns the other solids interval for the profile.
func (p ConstituentProfile) OtherSolidsInterval() Interval {
	return p.Components.OtherSolids
}

// TotalSugarInterval returns added sugars plus lactose.
func (p ConstituentProfile) TotalSugarInterval() Interval {
	return p.AddedSugarsInterval().Add(p.LactoseInterval())
}

// Composition returns the aggregated four-component composition for the profile.
func (p ConstituentProfile) Composition() Composition {
	return CompositionFromProfile(p)
}
