package creamery

import (
	"fmt"
	"slices"
	"strings"
)

type ingredientSlot struct {
	definition *Ingredient
	lot        Lot
	bounds     Interval
}

func canonicalLot(lot Lot, cache map[IngredientID]*Ingredient) (Lot, *Ingredient) {
	if cache == nil {
		cache = make(map[IngredientID]*Ingredient)
	}

	assignDefinition := func(def Ingredient) (Lot, *Ingredient) {
		definition := normalizeDefinition(def)
		if cached, ok := cache[definition.ID]; ok {
			lot.Definition = cached
			if lot.Label == "" {
				lot.Label = cached.Name
			}
			return lot, cached
		}
		cache[definition.ID] = &definition
		lot.Definition = &definition
		if lot.Label == "" {
			lot.Label = definition.Name
		}
		return lot, &definition
	}

	if lot.Definition != nil {
		return assignDefinition(*lot.Definition)
	}

	profile := lot.EffectiveProfile()
	def := Ingredient{
		ID:      profile.ID,
		Name:    lot.Label,
		Profile: profile,
	}
	return assignDefinition(def)
}

// Problem defines an ice cream formulation problem.
type Problem struct {
	Target FormulationTarget

	OrderConstraints bool
	Constraints      []LinearConstraint

	slots     []ingredientSlot
	specIndex map[IngredientID]int
}

// NewProblem creates a problem with the given specs and canonical target.
func NewProblem(specs []Ingredient, target FormulationTarget) *Problem {
	lots := make([]Lot, len(specs))
	for i, spec := range specs {
		lots[i] = spec.DefaultLot()
	}
	return NewFormulationProblem(lots, target)
}

// NewFormulationProblem creates a problem using the richer formulation target.
func NewFormulationProblem(lots []Lot, target FormulationTarget) *Problem {
	slots := make([]ingredientSlot, len(lots))
	specIndex := make(map[IngredientID]int, len(lots))
	defCache := make(map[IngredientID]*Ingredient, len(lots))
	for i, lot := range lots {
		normalizedLot, def := canonicalLot(lot, defCache)
		if def == nil {
			continue
		}
		slots[i] = ingredientSlot{
			definition: def,
			lot:        normalizedLot,
			bounds:     Range(0, 1),
		}
		specIndex[def.ID] = i
	}
	return &Problem{
		Target:      target,
		Constraints: make([]LinearConstraint, 0),
		slots:       slots,
		specIndex:   specIndex,
	}
}

// Specs returns a copy of the ingredient specs in order.
func (p *Problem) Specs() []Ingredient {
	specs := make([]Ingredient, len(p.slots))
	for i, slot := range p.slots {
		if slot.definition != nil {
			specs[i] = *slot.definition
		}
	}
	return specs
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
	ids := make([]IngredientID, len(p.slots))
	for i, slot := range p.slots {
		if slot.definition != nil {
			ids[i] = slot.definition.ID
		}
	}
	return ids
}

// IngredientNames returns the spec names in order (for display only).
func (p *Problem) IngredientNames() []string {
	names := make([]string, len(p.slots))
	for i, slot := range p.slots {
		if slot.definition != nil {
			names[i] = slot.definition.Name
		}
	}
	return names
}

func (p *Problem) profileForIndex(i int) ConstituentProfile {
	return p.slots[i].lot.EffectiveProfile()
}

func (p *Problem) specByID(id IngredientID) (Ingredient, bool) {
	idx, ok := p.specIndex[id]
	if !ok {
		return Ingredient{}, false
	}
	if p.slots[idx].definition == nil {
		return Ingredient{}, false
	}
	return *p.slots[idx].definition, true
}

// LotByID returns the registered ingredient lot for the given ID.
func (p *Problem) LotByID(id IngredientID) (Lot, bool) {
	if idx, ok := p.specIndex[id]; ok && idx >= 0 && idx < len(p.slots) {
		return p.slots[idx].lot, true
	}
	return Lot{}, false
}

// OverrideLots replaces default lots with the provided ones when the spec is present.
func (p *Problem) OverrideLots(lots map[IngredientID]Lot) {
	for id, lot := range lots {
		idx, ok := p.specIndex[id]
		if !ok {
			continue
		}
		slot := p.slots[idx]
		normalizedLot, _ := canonicalLot(lot, nil)
		if slot.definition != nil {
			normalizedLot.Definition = slot.definition
			if normalizedLot.Label == "" {
				normalizedLot.Label = slot.definition.Name
			}
		}
		p.slots[idx].lot = normalizedLot
	}
}

// Lots returns a copy of the problem's ingredient lots.
func (p *Problem) Lots() map[IngredientID]Lot {
	copy := make(map[IngredientID]Lot, len(p.slots))
	for _, slot := range p.slots {
		if slot.definition == nil {
			continue
		}
		copy[slot.definition.ID] = slot.lot
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
	idx, ok := p.specIndex[id]
	if !ok {
		return fmt.Errorf("unknown ingredient: %s", id)
	}
	p.slots[idx].bounds = Range(lo, hi)
	return nil
}

// SetMinWeight sets a minimum weight for an ingredient.
func (p *Problem) SetMinWeight(id IngredientID, min float64) error {
	idx, ok := p.specIndex[id]
	if !ok {
		return fmt.Errorf("unknown ingredient: %s", id)
	}
	current := p.slots[idx].bounds
	if current.Lo == 0 && current.Hi == 0 {
		current = Range(0, 1)
	}
	p.slots[idx].bounds = Range(min, current.Hi)
	return nil
}

// SetMaxWeight sets a maximum weight for an ingredient.
func (p *Problem) SetMaxWeight(id IngredientID, max float64) error {
	idx, ok := p.specIndex[id]
	if !ok {
		return fmt.Errorf("unknown ingredient: %s", id)
	}
	current := p.slots[idx].bounds
	if current.Lo == 0 && current.Hi == 0 {
		current = Range(0, 1)
	}
	p.slots[idx].bounds = Range(current.Lo, max)
	return nil
}

// IDByName returns the ingredient ID for a human-readable name.
func (p *Problem) IDByName(name string) (IngredientID, bool) {
	for _, slot := range p.slots {
		if slot.definition != nil && slot.definition.Name == name {
			return slot.definition.ID, true
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
	if len(p.slots) == 0 {
		return fmt.Errorf("no ingredients specified")
	}

	if err := p.Target.Validate(); err != nil {
		return fmt.Errorf("invalid target: %w", err)
	}

	seen := make(map[IngredientID]bool)
	for i, slot := range p.slots {
		if slot.definition == nil {
			return fmt.Errorf("ingredient %d missing definition", i)
		}
		id := slot.definition.ID
		if id == "" {
			return fmt.Errorf("ingredient %d missing ID", i)
		}
		if seen[id] {
			return fmt.Errorf("duplicate ingredient: %s", slot.definition.Name)
		}
		seen[id] = true

		if err := slot.lot.EffectiveProfile().Components.Validate(); err != nil {
			return fmt.Errorf("invalid ingredient %s: %w", slot.definition.Name, err)
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
	Blend      Blend
	Weights    map[IngredientID]float64
	Names      map[IngredientID]string
	Lots       map[IngredientID]Lot
	Achieved   ComponentFractions
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
	result += fmt.Sprintf("Achieved: %s", ComponentSummary(s.Achieved))
	return result
}

// RecipeComponents converts the solution weights to recipe components scaled
// to the provided total mass (defaults to 1 kg).
func (s Solution) RecipeComponents(totalMass float64) ([]RecipeComponent, error) {
	if totalMass <= 0 {
		totalMass = 1
	}
	if len(s.Weights) == 0 {
		return nil, fmt.Errorf("solution has no ingredient weights")
	}

	components := make([]RecipeComponent, 0, len(s.Weights))
	for id, weight := range s.Weights {
		if weight <= 0 {
			continue
		}
		lot, ok := s.Lots[id]
		if !ok || lot.Definition == nil {
			return nil, fmt.Errorf("solution missing lot definition for ingredient %s", id)
		}
		components = append(components, RecipeComponent{
			Ingredient: lot,
			MassKg:     weight * totalMass,
		})
	}
	if len(components) == 0 {
		return nil, fmt.Errorf("solution has no positive ingredient weights")
	}
	return components, nil
}

// Snapshot aggregates the solution into a BatchSnapshot using the provided
// mixing options (defaults applied internally).
func (s Solution) Snapshot(opts MixOptions) (BatchSnapshot, ProcessProperties, error) {
	components, err := s.RecipeComponents(1.0)
	if err != nil {
		return BatchSnapshot{}, ProcessProperties{}, err
	}
	return BuildProperties(components, opts)
}

// Score evaluates the solution against the provided preference curves.
func (s Solution) Score(pref RecipePreference, opts MixOptions) (float64, error) {
	pref = normalizeRecipePreference(pref)
	snapshot, process, err := s.Snapshot(opts)
	if err != nil {
		return 0, err
	}
	return pref.Score(snapshot, process), nil
}

// ImpliedMSNF calculates what MSNF interval a variable ingredient must have
// to achieve the target bounds, given the weights of all other ingredients.
func (s Solution) ImpliedMSNF(specs []Ingredient, target Interval, id IngredientID) (Interval, bool) {
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
	varMSNF := varSpec.Profile.MSNFInterval()

	var otherMSNF float64
	for _, spec := range specs {
		if spec.ID == id {
			continue
		}
		w := s.Weights[spec.ID]
		otherMSNF += w * spec.Profile.MSNFInterval().Mid()
	}

	requiredLo := (target.Lo - otherMSNF) / varWeight
	requiredHi := (target.Hi - otherMSNF) / varWeight

	possibleLo := varMSNF.Lo
	possibleHi := varMSNF.Hi

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
