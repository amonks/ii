package creamery

import "math"

// LabelGroup bundles ingredients that appear together on a consumer label.
type LabelGroup struct {
	Name                 string
	Keys                 []IngredientID
	FractionBounds       map[IngredientID]Interval
	EnforceInternalOrder bool
}

// LabelGoals captures macro targets inferred from a Nutrition Facts panel.
type LabelGoals struct {
	BatchMassKG      float64
	FatPct           float64
	SolidsPct        float64
	SweetnessPct     float64
	FreezingPointC   float64
	Overrun          float64
	ServeTemperature float64
	DrawTemperature  float64
	ShearRate        float64
	OverrunCap       *float64
}

// GoalsFromLabel converts Nutrition Facts + pint mass into formulation goals.
func GoalsFromLabel(facts NutritionFacts, pintMassGrams float64, serveTemp, drawTemp, shearRate float64) LabelGoals {
	if serveTemp == 0 {
		serveTemp = defaultServeTempC
	}
	if drawTemp == 0 {
		drawTemp = defaultDrawTempC
	}
	if shearRate == 0 {
		shearRate = defaultShearRate
	}

	massKg := pintMassGrams / 1000.0
	servings := pintMassGrams / facts.ServingSizeGrams
	fat := facts.TotalFatGrams * servings / 1000.0
	carbs := facts.TotalCarbGrams * servings / 1000.0
	sugars := facts.TotalSugarsGrams * servings / 1000.0
	protein := facts.ProteinGrams * servings / 1000.0
	ash := (facts.SodiumMg * servings / 1000.0) * (58.44 / 23.0) / 1000.0
	solids := fat + carbs + protein + ash
	water := math.Max(1e-6, massKg-solids)

	sucroseMoles := sugars * 1000.0 / mwSucrose
	fpEst := -kfWater * sucroseMoles / water
	finishedDensity := massKg / pintLiters
	overrunGuess := math.Max(0.0, math.Min(1.5, mixDensityKgPerL/finishedDensity-1.0))

	return LabelGoals{
		BatchMassKG:      100.0,
		FatPct:           fat / massKg,
		SolidsPct:        solids / massKg,
		SweetnessPct:     sugars / massKg,
		FreezingPointC:   fpEst,
		Overrun:          overrunGuess,
		ServeTemperature: serveTemp,
		DrawTemperature:  drawTemp,
		ShearRate:        shearRate,
	}
}

// ApplyGroupBounds enforces within-group fraction bounds.
func ApplyGroupBounds(p *Problem, groups []LabelGroup) {
	for _, group := range groups {
		if len(group.Keys) == 0 {
			continue
		}
		for keyID, bounds := range group.FractionBounds {
			keyName, ok := p.nameForID(keyID)
			if !ok {
				continue
			}
			if len(group.Keys) == 1 && group.Keys[0] == keyID {
				continue
			}
			// key <= hi * group_total
			if bounds.Hi < math.Inf(1) {
				coeffs := make(map[string]float64, len(group.Keys)+1)
				coeffs[keyName] = 1
				for _, memberID := range group.Keys {
					if memberName, ok := p.nameForID(memberID); ok {
						coeffs[memberName] -= bounds.Hi
					}
				}
				p.AddConstraint(coeffs, math.Inf(-1), 0, group.Name+":"+keyName+":hi")
			}
			// key >= lo * group_total
			if bounds.Lo > 0 {
				coeffs := make(map[string]float64, len(group.Keys)+1)
				coeffs[keyName] = 1
				for _, memberID := range group.Keys {
					if memberName, ok := p.nameForID(memberID); ok {
						coeffs[memberName] -= bounds.Lo
					}
				}
				p.AddConstraint(coeffs, 0, math.Inf(1), group.Name+":"+keyName+":lo")
			}
		}
	}
}

// ApplyLabelOrder enforces descending weight order of label groups (and optionally members).
func ApplyLabelOrder(p *Problem, groups []LabelGroup, epsilon float64) {
	if epsilon <= 0 {
		epsilon = 1e-3
	}
	for i := 0; i < len(groups)-1; i++ {
		earlier := groups[i]
		later := groups[i+1]
		if len(earlier.Keys) == 0 || len(later.Keys) == 0 {
			continue
		}
		coeffs := make(map[string]float64, len(earlier.Keys)+len(later.Keys))
		for _, key := range later.Keys {
			if name, ok := p.nameForID(key); ok {
				coeffs[name] = 1
			}
		}
		for _, key := range earlier.Keys {
			if name, ok := p.nameForID(key); ok {
				coeffs[name] -= 1
			}
		}
		p.AddConstraint(coeffs, math.Inf(-1), -epsilon, "label_order")
	}

	for _, group := range groups {
		if !group.EnforceInternalOrder {
			continue
		}
		for i := 0; i < len(group.Keys)-1; i++ {
			a := group.Keys[i]
			b := group.Keys[i+1]
			aName, aOK := p.nameForID(a)
			bName, bOK := p.nameForID(b)
			if !aOK || !bOK {
				continue
			}
			coeffs := map[string]float64{
				bName: 1,
				aName: -1,
			}
			p.AddConstraint(coeffs, math.Inf(-1), 0, group.Name+":internal")
		}
	}
}
