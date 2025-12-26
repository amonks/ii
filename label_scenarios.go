package creamery

import (
	"fmt"
	"math"
)

const (
	presenceFloorFraction = labelPercentEPS * 1e-3
	orderEpsilonFraction  = labelPercentEPS * 0.1
)

// LabelScenarioResult summarizes a solved label reconstruction.
type LabelScenarioResult struct {
	Name             string
	LabelIngredients []string
	LabelFacts       NutritionFacts
	PredictedFacts   NutritionFacts
	Goals            LabelGoals
	Problem          *Problem
	Solution         *Solution
	Recipe           *Recipe
	ServingSizeGrams float64
	Metrics          BatchSnapshot
	Process          ProcessProperties
	PintMassGrams    float64
	Specs            []Ingredient
	BatchDetails     map[IngredientID]Lot
}

type scenarioIngredients struct {
	catalog  IngredientCatalog
	batches  map[IngredientID]Lot
	specs    []Ingredient
	lots     []Lot
	nameToID map[string]IngredientID
}

func newScenarioIngredients() *scenarioIngredients {
	return &scenarioIngredients{
		catalog:  DefaultIngredientCatalog(),
		batches:  make(map[IngredientID]Lot),
		specs:    make([]Ingredient, 0),
		lots:     make([]Lot, 0),
		nameToID: make(map[string]IngredientID),
	}
}

func (s *scenarioIngredients) addClone(key, name string, override func(*Lot)) {
	base, ok := s.catalog.InstanceByKey(key)
	if !ok {
		return
	}
	inst := base
	if name != "" {
		inst = renameInstance(inst, name)
	}
	if override != nil {
		override(&inst)
	}
	s.addDetail(inst)
}

func (s *scenarioIngredients) addDetail(inst Lot) {
	profile := inst.EffectiveProfile()
	s.nameToID[profile.Name] = profile.ID
	spec := Ingredient{}
	if inst.Definition != nil {
		spec = *inst.Definition
	}
	spec.Profile = profile
	spec.ID = profile.ID
	spec.Name = profile.Name
	inst = inst.WithSpec(spec)
	inst.Label = profile.Name
	s.batches[profile.ID] = inst
	s.specs = append(s.specs, spec)
	s.lots = append(s.lots, inst)
}

func (s *scenarioIngredients) Specs() []Ingredient {
	return s.specs
}

func (s *scenarioIngredients) Lots() []Lot {
	result := make([]Lot, len(s.lots))
	copy(result, s.lots)
	return result
}

func (s *scenarioIngredients) Batches() map[IngredientID]Lot {
	copy := make(map[IngredientID]Lot, len(s.batches))
	for id, batch := range s.batches {
		copy[id] = batch
	}
	return copy
}

func renameInstance(inst Lot, name string) Lot {
	profile := inst.EffectiveProfile()
	profile.Name = name
	profile.ID = NewIngredientID(name)
	spec := SpecFromProfile(profile)
	inst = inst.WithSpec(spec)
	inst.Label = name
	return inst
}

func (s *scenarioIngredients) id(name string) IngredientID {
	if id, ok := s.nameToID[name]; ok {
		return id
	}
	return NewIngredientID(name)
}

func (s *scenarioIngredients) idList(names ...string) []IngredientID {
	result := make([]IngredientID, 0, len(names))
	for _, name := range names {
		result = append(result, s.id(name))
	}
	return result
}

// SolveBenAndJerryVanilla recreates the Ben & Jerry's Vanilla label problem.
func SolveBenAndJerryVanilla() (*LabelScenarioResult, error) {
	return SolveLabelScenarioByKey(LabelBenAndJerryVanilla)
}

func SolveJenisSweetCream() (*LabelScenarioResult, error) {
	return SolveLabelScenarioByKey(LabelJenisSweetCream)
}

func SolveHaagenDazsVanilla() (*LabelScenarioResult, error) {
	return SolveLabelScenarioByKey(LabelHaagenDazsVanilla)
}

func SolveBrighamsVanilla() (*LabelScenarioResult, error) {
	return SolveLabelScenarioByKey(LabelBrighamsVanilla)
}

func SolveBreyersVanilla() (*LabelScenarioResult, error) {
	return SolveLabelScenarioByKey(LabelBreyersVanilla)
}

func SolveTalentiVanilla() (*LabelScenarioResult, error) {
	return SolveLabelScenarioByKey(LabelTalentiVanilla)
}

func SolveLabelScenarioByKey(key string) (*LabelScenarioResult, error) {
	label, ok := FDALabelByKey(key)
	if !ok {
		return nil, fmt.Errorf("unknown label scenario %q", key)
	}
	return SolveFDALabel(label, DefaultIngredientCatalog())
}

// ingredientAliases maps DSL ingredient names to catalog keys.
var ingredientAliases = map[string]string{
	"cane_sugar": "sucrose",
	"sugar":      "sucrose",
}

func catalogKeyForIngredient(id string) string {
	if alias, ok := ingredientAliases[id]; ok {
		return alias
	}
	return id
}

// SolveFDALabel solves a label reconstruction problem directly from an FDA Label.
func SolveFDALabel(label Label, catalog IngredientCatalog) (*LabelScenarioResult, error) {
	// Build ingredient lots from the label
	builder := newScenarioIngredients()
	builder.catalog = catalog

	for _, ing := range label.Ingredients {
		catalogKey := catalogKeyForIngredient(ing.ID)
		builder.addClone(catalogKey, ing.ID, func(inst *Lot) {
			if len(ing.Components) > 0 && inst.Definition != nil {
				def := *inst.Definition
				for key, value := range ing.Components {
					switch key {
					case "water":
						def.Profile.Components.Water = Point(value)
					case "fat":
						def.Profile.Components.Fat = Point(value)
					case "protein":
						def.Profile.Components.Protein = Point(value)
					case "lactose":
						def.Profile.Components.Lactose = Point(value)
					case "sucrose":
						def.Profile.Components.Sucrose = Point(value)
					case "glucose":
						def.Profile.Components.Glucose = Point(value)
					case "fructose":
						def.Profile.Components.Fructose = Point(value)
					case "other_solids":
						def.Profile.Components.OtherSolids = Point(value)
					}
				}
				*inst = inst.WithSpec(def)
			}
		})
	}

	lots := builder.Lots()
	if len(lots) == 0 {
		return nil, fmt.Errorf("label %s has no ingredients", label.Name)
	}

	// Convert LabelFacts to NutritionFacts
	facts := NutritionFacts{
		ServingSizeGrams:  label.Facts.ServingSizeGrams,
		Calories:          label.Facts.Calories,
		TotalFatGrams:     label.Facts.TotalFatGrams,
		SaturatedFatGrams: label.Facts.SaturatedFatGrams,
		TransFatGrams:     label.Facts.TransFatGrams,
		CholesterolMg:     label.Facts.CholesterolMg,
		TotalCarbGrams:    label.Facts.TotalCarbGrams,
		TotalSugarsGrams:  label.Facts.TotalSugarsGrams,
		AddedSugarsGrams:  label.Facts.AddedSugarsGrams,
		ProteinGrams:      label.Facts.ProteinGrams,
		SodiumMg:          label.Facts.SodiumMg,
	}

	nutritionLabel := nutritionLabelFromFacts(facts)
	target := nutritionLabel.ToTarget()
	target.POD = Interval{}
	target.PAC = Interval{}

	problem := NewFormulationProblem(lots, target)
	problem.OverrideLots(builder.Batches())

	// Set presence floor for all ingredients
	presence := make([]IngredientID, len(label.Ingredients))
	for i, ing := range label.Ingredients {
		presence[i] = NewIngredientID(ing.ID)
	}
	if err := setPresenceFloor(problem, presence); err != nil {
		return nil, err
	}

	// Convert FDA groups to LabelGroups and apply constraints
	groups := convertFDAGroups(label, builder)
	if len(groups) > 0 {
		ApplyGroupBounds(problem, groups)
		ApplyLabelOrder(problem, groups, labelOrderEps())
	} else {
		problem.OrderConstraints = true
	}

	if err := problem.Validate(); err != nil {
		return nil, fmt.Errorf("invalid label problem for %s: %w", label.Name, err)
	}

	pintMass := label.PintMassGrams
	if pintMass == 0 {
		pintMass = facts.ServingSizeGrams * 3
	}

	goals := GoalsFromLabel(facts, pintMass, defaultServeTempC, defaultDrawTempC, defaultShearRate)

	solver, err := NewSolver(problem)
	if err != nil {
		return nil, err
	}
	solution, err := solver.FindSolution()
	if err != nil {
		return nil, fmt.Errorf("%s LP infeasible: %w", label.Name, err)
	}

	specs := builder.Specs()
	recipe, predicted, serving, metrics, process, err := recipeFromSolution(solution, specs, goals, facts.SodiumMg)
	if err != nil {
		return nil, fmt.Errorf("unable to build recipe for %s: %w", label.Name, err)
	}

	batchDetails := make(map[IngredientID]Lot, len(solution.Lots))
	for id, lot := range solution.Lots {
		batchDetails[id] = lot
	}

	return &LabelScenarioResult{
		Name:             label.Name,
		LabelIngredients: ingredientNames(label.Ingredients),
		LabelFacts:       facts,
		PredictedFacts:   predicted,
		Goals:            goals,
		Problem:          problem,
		Solution:         solution,
		Recipe:           recipe,
		ServingSizeGrams: serving,
		Metrics:          metrics,
		Process:          process,
		PintMassGrams:    pintMass,
		Specs:            specs,
		BatchDetails:     batchDetails,
	}, nil
}

func convertFDAGroups(label Label, builder *scenarioIngredients) []LabelGroup {
	groups := make([]LabelGroup, 0, len(label.Groups)+len(label.Ingredients))

	// Track which ingredients are in explicit groups
	inGroup := make(map[string]bool)
	for _, g := range label.Groups {
		for _, member := range g.Members {
			inGroup[member] = true
		}
	}

	// Convert explicit groups
	for _, g := range label.Groups {
		keys := make([]IngredientID, len(g.Members))
		for i, member := range g.Members {
			keys[i] = builder.id(member)
		}
		group := LabelGroup{
			Name:                 g.Name,
			Keys:                 keys,
			EnforceInternalOrder: g.EnforceOrder,
		}
		if len(g.FractionBounds) > 0 {
			group.FractionBounds = make(map[IngredientID]Interval)
			for key, bound := range g.FractionBounds {
				group.FractionBounds[builder.id(key)] = RangeWithEps(bound.Lo, bound.Hi)
			}
		}
		groups = append(groups, group)
	}

	// Create singleton groups for ingredients not in explicit groups
	for _, ing := range label.Ingredients {
		if !inGroup[ing.ID] {
			groups = append(groups, LabelGroup{
				Name: ing.ID,
				Keys: []IngredientID{builder.id(ing.ID)},
			})
		}
	}

	return groups
}

func ingredientNames(ingredients []LabelIngredient) []string {
	names := make([]string, len(ingredients))
	for i, ing := range ingredients {
		names[i] = ing.ID
	}
	return names
}

func setPresenceFloor(p *Problem, ids []IngredientID) error {
	for _, id := range ids {
		if err := p.SetMinWeight(id, presenceFloorFraction); err != nil {
			return err
		}
	}
	return nil
}

func labelOrderEps() float64 {
	return math.Max(1e-6, orderEpsilonFraction)
}

// RangeWithEps expands a [lo, hi] range using labelPercentEPS slack.
func RangeWithEps(lo, hi float64) Interval {
	return Interval{
		Lo: math.Max(0, lo*(1-labelPercentEPS)),
		Hi: math.Min(1, hi*(1+labelPercentEPS)),
	}
}

// SolveAllLabelScenarios solves every built-in label scenario.
func SolveAllLabelScenarios() ([]*LabelScenarioResult, error) {
	builders := []func() (*LabelScenarioResult, error){
		SolveBenAndJerryVanilla,
		SolveJenisSweetCream,
		SolveHaagenDazsVanilla,
		SolveBrighamsVanilla,
		SolveBreyersVanilla,
		SolveTalentiVanilla,
	}

	results := make([]*LabelScenarioResult, 0, len(builders))
	for _, build := range builders {
		res, err := build()
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}
