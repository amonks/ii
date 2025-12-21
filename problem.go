package creamery

import (
	"fmt"
	"slices"
)

// Problem defines an ice cream formulation problem.
type Problem struct {
	// Ingredients available for use (order matters for labeling constraints)
	Ingredients []Ingredient

	// Target composition to achieve
	Target FormulationTarget

	// TargetPOD is the target sweetening power range (optional).
	// If non-zero, constrains the total POD (including lactose from MSNF).
	TargetPOD Interval

	// TargetPAC is the target anti-freezing power range (optional).
	// If non-zero, constrains the total PAC (including lactose from MSNF).
	TargetPAC Interval

	// WeightBounds constrains individual ingredient weights (optional)
	// Keys are ingredient names, values are [min, max] weight fractions
	WeightBounds map[string]Interval

	// OrderConstraints specifies that ingredients must be in descending
	// weight order as listed. This reflects FDA labeling requirements.
	// If true, Ingredients[0] >= Ingredients[1] >= ... by weight.
	OrderConstraints bool

	// Additional linear constraints of the form lower <= sum(coeff_i * w_i) <= upper.
	Constraints []LinearConstraint
}

// NewProblem creates a problem with the given ingredients and legacy composition target.
func NewProblem(ingredients []Ingredient, target Composition) *Problem {
	return NewFormulationProblem(ingredients, FormulationFromComposition(target))
}

// NewFormulationProblem creates a problem using the richer formulation target.
func NewFormulationProblem(ingredients []Ingredient, target FormulationTarget) *Problem {
	return &Problem{
		Ingredients:  ingredients,
		Target:       target,
		TargetPOD:    target.POD,
		TargetPAC:    target.PAC,
		WeightBounds: make(map[string]Interval),
	}
}

// LinearConstraint represents a linear expression over ingredient weights.
type LinearConstraint struct {
	Coeffs map[string]float64
	Lower  float64
	Upper  float64
	Note   string
}

// SetWeightBound constrains an ingredient's weight to [lo, hi].
func (p *Problem) SetWeightBound(name string, lo, hi float64) error {
	found := false
	for _, ing := range p.Ingredients {
		if ing.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown ingredient: %s", name)
	}
	p.WeightBounds[name] = Range(lo, hi)
	return nil
}

// SetMinWeight sets a minimum weight for an ingredient.
func (p *Problem) SetMinWeight(name string, min float64) error {
	bound, ok := p.WeightBounds[name]
	if !ok {
		bound = Range(0, 1)
	}
	return p.SetWeightBound(name, min, bound.Hi)
}

// SetMaxWeight sets a maximum weight for an ingredient.
func (p *Problem) SetMaxWeight(name string, max float64) error {
	bound, ok := p.WeightBounds[name]
	if !ok {
		bound = Range(0, 1)
	}
	return p.SetWeightBound(name, bound.Lo, max)
}

// Validate checks that the problem is well-formed.
func (p *Problem) Validate() error {
	if len(p.Ingredients) == 0 {
		return fmt.Errorf("no ingredients specified")
	}

	if err := p.Target.Composition.Valid(); err != nil {
		return fmt.Errorf("invalid target: %w", err)
	}

	names := make(map[string]bool)
	for _, ing := range p.Ingredients {
		if names[ing.Name] {
			return fmt.Errorf("duplicate ingredient: %s", ing.Name)
		}
		names[ing.Name] = true

		if err := ing.Comp.Valid(); err != nil {
			return fmt.Errorf("invalid ingredient %s: %w", ing.Name, err)
		}
	}

	for name := range p.WeightBounds {
		if !names[name] {
			return fmt.Errorf("weight bound for unknown ingredient: %s", name)
		}
	}

	for i, constraint := range p.Constraints {
		for name := range constraint.Coeffs {
			if !names[name] {
				return fmt.Errorf("constraint %d references unknown ingredient %s", i, name)
			}
		}
		if constraint.Lower > constraint.Upper {
			return fmt.Errorf("constraint %d has lower > upper", i)
		}
	}

	return nil
}

// IngredientNames returns the names of ingredients in order.
func (p *Problem) IngredientNames() []string {
	names := make([]string, len(p.Ingredients))
	for i, ing := range p.Ingredients {
		names[i] = ing.Name
	}
	return names
}

// AddConstraint appends a linear constraint of the form lower <= sum(coeff_i * w_i) <= upper.
func (p *Problem) AddConstraint(coeffs map[string]float64, lower, upper float64, note string) {
	copied := make(map[string]float64, len(coeffs))
	for k, v := range coeffs {
		copied[k] = v
	}
	p.Constraints = append(p.Constraints, LinearConstraint{
		Coeffs: copied,
		Lower:  lower,
		Upper:  upper,
		Note:   note,
	})
}

// Solution represents a feasible (or partial) solution to a Problem.
type Solution struct {
	// Weights maps ingredient names to their weight fractions.
	Weights map[string]float64

	// Achieved is the composition achieved by this solution.
	Achieved Composition
}

// String returns a human-readable representation.
func (s Solution) String() string {
	names := make([]string, 0, len(s.Weights))
	for name := range s.Weights {
		names = append(names, name)
	}
	slices.Sort(names)

	result := "Recipe:\n"
	for _, name := range names {
		w := s.Weights[name]
		if w > 0.001 { // skip negligible amounts
			result += fmt.Sprintf("  %s: %.2f%%\n", name, w*100)
		}
	}
	result += fmt.Sprintf("Achieved: %s", s.Achieved)
	return result
}

// Bounds represents the feasible range for each ingredient weight.
type Bounds struct {
	// WeightRanges maps ingredient names to their feasible weight intervals.
	WeightRanges map[string]Interval

	// Feasible is true if any solution exists.
	Feasible bool
}

// ImpliedComposition calculates what composition a variable ingredient must have
// to achieve the target, given the weights of all other ingredients.
// Returns the required MSNF fraction for the named ingredient.
// This is useful for ingredients like "Nonfat Milk" that could be any concentration.
func (s Solution) ImpliedMSNF(ingredients []Ingredient, target Composition, name string) (Interval, bool) {
	// Find the ingredient
	var varIngredient Ingredient
	varWeight := s.Weights[name]
	found := false
	for _, ing := range ingredients {
		if ing.Name == name {
			varIngredient = ing
			found = true
			break
		}
	}
	if !found || varWeight < 0.001 {
		return Interval{}, false
	}

	// Calculate MSNF contribution from all OTHER ingredients
	var otherMSNF float64
	for _, ing := range ingredients {
		if ing.Name == name {
			continue
		}
		w := s.Weights[ing.Name]
		otherMSNF += w * ing.Comp.MSNF.Mid()
	}

	// Required MSNF from the variable ingredient
	// target.MSNF = otherMSNF + varWeight * varMSNF
	// varMSNF = (target.MSNF - otherMSNF) / varWeight
	requiredLo := (target.MSNF.Lo - otherMSNF) / varWeight
	requiredHi := (target.MSNF.Hi - otherMSNF) / varWeight

	// Clamp to ingredient's possible range
	possibleLo := varIngredient.Comp.MSNF.Lo
	possibleHi := varIngredient.Comp.MSNF.Hi

	impliedLo := max(requiredLo, possibleLo)
	impliedHi := min(requiredHi, possibleHi)

	if impliedLo > impliedHi {
		return Interval{}, false // impossible
	}

	return Interval{Lo: impliedLo, Hi: impliedHi}, true
}

// DescribeNonfatMilk interprets an MSNF fraction as a practical description.
// Returns a human-readable string describing what form the nonfat milk is in.
func DescribeNonfatMilk(msnf Interval) string {
	mid := msnf.Mid()

	// Liquid skim milk: ~9% MSNF
	// Evaporated skim: ~20% MSNF
	// Condensed (no sugar): ~25-30% MSNF
	// Dry powder: ~97% MSNF

	switch {
	case mid < 0.12:
		return fmt.Sprintf("liquid skim milk (%.0f%% MSNF)", mid*100)
	case mid < 0.25:
		waterPct := 100 - mid*100
		return fmt.Sprintf("concentrated skim (~%.0f%% water, %.0f%% MSNF)", waterPct, mid*100)
	case mid < 0.50:
		// Reconstituted: X parts water to 1 part powder
		// If final is Y% MSNF and powder is 97% MSNF:
		// Y = 97 / (1 + X) => X = 97/Y - 1
		ratio := 0.97/mid - 1
		return fmt.Sprintf("reconstituted NFDM (~%.1f:1 water:powder, %.0f%% MSNF)", ratio, mid*100)
	case mid < 0.85:
		ratio := 0.97/mid - 1
		return fmt.Sprintf("lightly reconstituted NFDM (~%.1f:1 water:powder, %.0f%% MSNF)", ratio, mid*100)
	default:
		return fmt.Sprintf("nonfat dry milk powder (%.0f%% MSNF)", mid*100)
	}
}

// String returns a human-readable representation.
func (b Bounds) String() string {
	if !b.Feasible {
		return "No feasible solution"
	}

	names := make([]string, 0, len(b.WeightRanges))
	for name := range b.WeightRanges {
		names = append(names, name)
	}
	slices.Sort(names)

	result := "Feasible ranges:\n"
	for _, name := range names {
		r := b.WeightRanges[name]
		result += fmt.Sprintf("  %s: [%.2f%%, %.2f%%]\n", name, r.Lo*100, r.Hi*100)
	}
	return result
}
