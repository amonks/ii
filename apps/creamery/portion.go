package creamery

// Portion represents the fractional contribution of a lot to a mix (0-1).
type Portion struct {
	Lot      Lot
	Fraction float64
}

// PortionMass couples a lot with an absolute mass (kg) used at IO boundaries.
type PortionMass struct {
	Lot    Lot
	MassKg float64
}

// NormalizePortions scales fractions so they sum to 1, dropping zero/negative entries.
func NormalizePortions(portions []Portion) []Portion {
	sum := 0.0
	for _, portion := range portions {
		if portion.Fraction > 0 {
			sum += portion.Fraction
		}
	}
	if sum <= 0 {
		return nil
	}
	inv := 1 / sum
	normalized := make([]Portion, 0, len(portions))
	for _, portion := range portions {
		if portion.Fraction <= 0 {
			continue
		}
		portion.Fraction *= inv
		normalized = append(normalized, portion)
	}
	return normalized
}

// PortionsFromMasses converts absolute masses to normalized fractions.
func PortionsFromMasses(masses []PortionMass) ([]Portion, float64) {
	total := 0.0
	for _, mass := range masses {
		if mass.MassKg > 0 {
			total += mass.MassKg
		}
	}
	if total <= 0 {
		return nil, 0
	}
	inv := 1 / total
	portions := make([]Portion, 0, len(masses))
	for _, mass := range masses {
		if mass.MassKg <= 0 {
			continue
		}
		portions = append(portions, Portion{
			Lot:      mass.Lot,
			Fraction: mass.MassKg * inv,
		})
	}
	return portions, total
}

// PortionsToMasses scales fractions into absolute masses using the provided total.
func PortionsToMasses(portions []Portion, totalMass float64) []PortionMass {
	if totalMass <= 0 {
		totalMass = 1
	}
	masses := make([]PortionMass, 0, len(portions))
	for _, portion := range portions {
		if portion.Fraction <= 0 {
			continue
		}
		masses = append(masses, PortionMass{
			Lot:    portion.Lot,
			MassKg: portion.Fraction * totalMass,
		})
	}
	return masses
}
