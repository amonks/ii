package creamery

import (
	"fmt"
	"slices"
	"strings"
)

// Problem defines an ice cream formulation problem.
type Problem struct {
	Specs []IngredientSpec

	Target FormulationTarget

	WeightBounds     map[IngredientID]Interval
	OrderConstraints bool
	Constraints      []LinearConstraint

	specIndex map[IngredientID]int
	profiles  []ConstituentProfile
	lots      []IngredientLot
}

// NewProblem creates a problem with the given specs and legacy composition target.
func NewProblem(specs []IngredientSpec, target Composition) *Problem {
	lots := make([]IngredientLot, len(specs))
	for i, spec := range specs {
		lots[i] = NewIngredientLot(spec)
	}
	return NewFormulationProblem(lots, FormulationFromComposition(target))
}

// NewFormulationProblem creates a problem using the richer formulation target.
func NewFormulationProblem(lots []IngredientLot, target FormulationTarget) *Problem {
	canonicalLots := make([]IngredientLot, len(lots))
	canonicalSpecs := make([]IngredientSpec, len(lots))
	specIndex := make(map[IngredientID]int, len(lots))
	profiles := make([]ConstituentProfile, len(lots))
	for i, lot := range lots {
		spec := lot.Ingredient
		if spec.ID == "" {
			spec.ID = NewIngredientID(spec.Name)
		}
		if spec.Name == "" {
			spec.Name = spec.ID.String()
		}
		lot.Ingredient = spec
		profile := lot.EffectiveProfile()
		lot.Profile = profile
		canonicalLots[i] = lot
		canonicalSpecs[i] = spec
		specIndex[spec.ID] = i
		profiles[i] = profile
	}
	return &Problem{
		Specs:        canonicalSpecs,
		Target:       target,
		WeightBounds: make(map[IngredientID]Interval),
		Constraints:  make([]LinearConstraint, 0),
		specIndex:    specIndex,
		profiles:     profiles,
		lots:         canonicalLots,
	}
}

// LinearConstraint represents a linear expression over ingredient weights.
type LinearConstraint struct {
	Coeffs map[IngredientID]float64
	Lower  float64
	Upper  float64
	Note   string
}

// IngredientIDs returns the spec IDs in order.
func (p *Problem) IngredientIDs() []IngredientID {
	ids := make([]IngredientID, len(p.Specs))
	for i, spec := range p.Specs {
		ids[i] = spec.ID
	}
	return ids
}

// IngredientNames returns the spec names in order (for display only).
func (p *Problem) IngredientNames() []string {
	names := make([]string, len(p.Specs))
	for i, spec := range p.Specs {
		names[i] = spec.Name
	}
	return names
}

func (p *Problem) compositionForIndex(i int) Composition {
	return p.profiles[i].Composition()
}

func (p *Problem) profileForIndex(i int) ConstituentProfile {
	return p.profiles[i]
}

func (p *Problem) specByID(id IngredientID) (IngredientSpec, bool) {
	idx, ok := p.specIndex[id]
	if !ok {
		return IngredientSpec{}, false
	}
	return p.Specs[idx], true
}

// LotByID returns the registered ingredient lot for the given ID.
func (p *Problem) LotByID(id IngredientID) (IngredientLot, bool) {
	if idx, ok := p.specIndex[id]; ok && idx >= 0 && idx < len(p.lots) {
		return p.lots[idx], true
	}
	return IngredientLot{}, false
}

// OverrideLots replaces default lots with the provided ones when the spec is present.
func (p *Problem) OverrideLots(lots map[IngredientID]IngredientLot) {
	for id, lot := range lots {
		idx, ok := p.specIndex[id]
		if !ok {
			continue
		}
		spec := p.Specs[idx]
		if lot.Ingredient.ID == "" {
			lot.Ingredient.ID = spec.ID
		}
		if lot.Ingredient.Name == "" {
			lot.Ingredient.Name = spec.Name
		}
		profile := lot.EffectiveProfile()
		lot.Profile = profile
		p.lots[idx] = lot
		p.profiles[idx] = profile
		p.Specs[idx] = lot.Ingredient
	}
}

// Lots returns a copy of the problem's ingredient lots.
func (p *Problem) Lots() map[IngredientID]IngredientLot {
	copy := make(map[IngredientID]IngredientLot, len(p.lots))
	for _, lot := range p.lots {
		copy[lot.Ingredient.ID] = lot
	}
	return copy
}

func (p *Problem) nameForID(id IngredientID) string {
	if spec, ok := p.specByID(id); ok {
		return spec.Name
	}
	return id.String()
}

// SetWeightBound constrains an ingredient's weight to [lo, hi].
func (p *Problem) SetWeightBound(id IngredientID, lo, hi float64) error {
	if _, ok := p.specIndex[id]; !ok {
		return fmt.Errorf("unknown ingredient: %s", id)
	}
	p.WeightBounds[id] = Range(lo, hi)
	return nil
}

// SetMinWeight sets a minimum weight for an ingredient.
func (p *Problem) SetMinWeight(id IngredientID, min float64) error {
	bound, ok := p.WeightBounds[id]
	if !ok {
		bound = Range(0, 1)
	}
	return p.SetWeightBound(id, min, bound.Hi)
}

// SetMaxWeight sets a maximum weight for an ingredient.
func (p *Problem) SetMaxWeight(id IngredientID, max float64) error {
	bound, ok := p.WeightBounds[id]
	if !ok {
		bound = Range(0, 1)
	}
	return p.SetWeightBound(id, bound.Lo, max)
}

// IDByName returns the ingredient ID for a human-readable name.
func (p *Problem) IDByName(name string) (IngredientID, bool) {
	for _, spec := range p.Specs {
		if spec.Name == name {
			return spec.ID, true
		}
	}
	return "", false
}

// SetMinWeightByName is a convenience wrapper around SetMinWeight.
func (p *Problem) SetMinWeightByName(name string, min float64) error {
	id, ok := p.IDByName(name)
	if !ok {
		return fmt.Errorf("unknown ingredient: %s", name)
	}
	return p.SetMinWeight(id, min)
}

// SetMaxWeightByName is a convenience wrapper around SetMaxWeight.
func (p *Problem) SetMaxWeightByName(name string, max float64) error {
	id, ok := p.IDByName(name)
	if !ok {
		return fmt.Errorf("unknown ingredient: %s", name)
	}
	return p.SetMaxWeight(id, max)
}

// Validate checks that the problem is well-formed.
func (p *Problem) Validate() error {
	if len(p.Specs) == 0 {
		return fmt.Errorf("no ingredients specified")
	}

	if err := p.Target.Validate(); err != nil {
		return fmt.Errorf("invalid target: %w", err)
	}

	seen := make(map[IngredientID]bool)
	for i, spec := range p.Specs {
		if spec.ID == "" {
			return fmt.Errorf("ingredient %d missing ID", i)
		}
		if seen[spec.ID] {
			return fmt.Errorf("duplicate ingredient: %s", spec.Name)
		}
		seen[spec.ID] = true

		profile := p.profileForIndex(i)
		if err := profile.Composition().Valid(); err != nil {
			return fmt.Errorf("invalid ingredient %s: %w", spec.Name, err)
		}
	}

	for id := range p.WeightBounds {
		if !seen[id] {
			return fmt.Errorf("weight bound for unknown ingredient: %s", id)
		}
	}

	for i, constraint := range p.Constraints {
		for id := range constraint.Coeffs {
			if !seen[id] {
				return fmt.Errorf("constraint %d references unknown ingredient %s", i, id)
			}
		}
		if constraint.Lower > constraint.Upper {
			return fmt.Errorf("constraint %d has lower > upper", i)
		}
	}

	return nil
}

// AddConstraint appends a linear constraint of the form lower <= sum(coeff_i * w_i) <= upper.
func (p *Problem) AddConstraint(coeffs map[IngredientID]float64, lower, upper float64, note string) {
	copied := make(map[IngredientID]float64, len(coeffs))
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
	Weights    map[IngredientID]float64
	Names      map[IngredientID]string
	Lots       map[IngredientID]IngredientLot
	Achieved   Composition
	Components ConstituentComponents
}

// String returns a human-readable representation.
func (s Solution) String() string {
	if len(s.Weights) == 0 {
		return "Recipe: <empty>"
	}
	type entry struct {
		name   string
		weight float64
	}
	entries := make([]entry, 0, len(s.Weights))
	for id, w := range s.Weights {
		name := id.String()
		if s.Names != nil && s.Names[id] != "" {
			name = s.Names[id]
		}
		entries = append(entries, entry{name: name, weight: w})
	}
	slices.SortFunc(entries, func(a, b entry) int {
		if a.name < b.name {
			return -1
		}
		if a.name > b.name {
			return 1
		}
		return 0
	})
	result := "Recipe:\n"
	for _, e := range entries {
		if e.weight > 0.001 {
			result += fmt.Sprintf("  %s: %.2f%%\n", e.name, e.weight*100)
		}
	}
	result += fmt.Sprintf("Achieved: %s", s.Achieved)
	return result
}

// ImpliedMSNF calculates what composition a variable ingredient must have
// to achieve the target, given the weights of all other ingredients.
func (s Solution) ImpliedMSNF(specs []IngredientSpec, target Composition, id IngredientID) (Interval, bool) {
	varSpecIndex := -1
	for i, spec := range specs {
		if spec.ID == id {
			varSpecIndex = i
			break
		}
	}
	if varSpecIndex == -1 {
		return Interval{}, false
	}

	varWeight := s.Weights[id]
	if varWeight < 0.001 {
		return Interval{}, false
	}

	varSpec := specs[varSpecIndex]
	varComp := CompositionFromProfile(varSpec.Profile)

	var otherMSNF float64
	for _, spec := range specs {
		if spec.ID == id {
			continue
		}
		w := s.Weights[spec.ID]
		comp := CompositionFromProfile(spec.Profile)
		otherMSNF += w * comp.MSNF.Mid()
	}

	requiredLo := (target.MSNF.Lo - otherMSNF) / varWeight
	requiredHi := (target.MSNF.Hi - otherMSNF) / varWeight

	possibleLo := varComp.MSNF.Lo
	possibleHi := varComp.MSNF.Hi

	impliedLo := max(requiredLo, possibleLo)
	impliedHi := min(requiredHi, possibleHi)

	if impliedLo > impliedHi {
		return Interval{}, false
	}

	return Interval{Lo: impliedLo, Hi: impliedHi}, true
}

// Bounds represents the feasible range for each ingredient weight.
type Bounds struct {
	WeightRanges map[IngredientID]Interval
	Names        map[IngredientID]string
	Feasible     bool
}

func (b Bounds) displayName(id IngredientID) string {
	if b.Names != nil && b.Names[id] != "" {
		return b.Names[id]
	}
	return id.String()
}

// String returns a human-readable representation of feasible ranges.
func (b Bounds) String() string {
	if !b.Feasible {
		return "No feasible solution"
	}

	ids := make([]IngredientID, 0, len(b.WeightRanges))
	for id := range b.WeightRanges {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, idB IngredientID) int {
		nameA := b.displayName(a)
		nameB := b.displayName(idB)
		return strings.Compare(nameA, nameB)
	})

	var builder strings.Builder
	builder.WriteString("Feasible ranges:\n")
	for _, id := range ids {
		r := b.WeightRanges[id]
		builder.WriteString(fmt.Sprintf("  %s: [%.2f%%, %.2f%%]\n", b.displayName(id), r.Lo*100, r.Hi*100))
	}
	return builder.String()
}

// DescribeNonfatMilk interprets an MSNF fraction as a practical description.
func DescribeNonfatMilk(msnf Interval) string {
	mid := msnf.Mid()

	switch {
	case mid < 0.12:
		return fmt.Sprintf("liquid skim milk (%.0f%% MSNF)", mid*100)
	case mid < 0.25:
		waterPct := 100 - mid*100
		return fmt.Sprintf("concentrated skim (~%.0f%% water, %.0f%% MSNF)", waterPct, mid*100)
	case mid < 0.50:
		ratio := 0.97/mid - 1
		return fmt.Sprintf("reconstituted NFDM (~%.1f:1 water:powder, %.0f%% MSNF)", ratio, mid*100)
	case mid < 0.85:
		ratio := 0.97/mid - 1
		return fmt.Sprintf("lightly reconstituted NFDM (~%.1f:1 water:powder, %.0f%% MSNF)", ratio, mid*100)
	default:
		return fmt.Sprintf("nonfat dry milk powder (%.0f%% MSNF)", mid*100)
	}
}
