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
	PintMassGrams    float64
	Specs            []IngredientDefinition
	BatchDetails     map[IngredientID]LotDescriptor
}

type scenarioIngredients struct {
	catalog  IngredientCatalog
	batches  map[IngredientID]LotDescriptor
	specs    []IngredientDefinition
	lots     []LotDescriptor
	nameToID map[string]IngredientID
}

func newScenarioIngredients() *scenarioIngredients {
	return &scenarioIngredients{
		catalog:  DefaultIngredientCatalog(),
		batches:  make(map[IngredientID]LotDescriptor),
		specs:    make([]IngredientDefinition, 0),
		lots:     make([]LotDescriptor, 0),
		nameToID: make(map[string]IngredientID),
	}
}

func (s *scenarioIngredients) addClone(key, name string, override func(*LotDescriptor)) {
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

func (s *scenarioIngredients) addDetail(inst LotDescriptor) {
	profile := inst.EffectiveProfile()
	s.nameToID[profile.Name] = profile.ID
	spec := IngredientDefinition{}
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

func (s *scenarioIngredients) Specs() []IngredientDefinition {
	return s.specs
}

func (s *scenarioIngredients) Lots() []LotDescriptor {
	result := make([]LotDescriptor, len(s.lots))
	copy(result, s.lots)
	return result
}

func (s *scenarioIngredients) Batches() map[IngredientID]LotDescriptor {
	copy := make(map[IngredientID]LotDescriptor, len(s.batches))
	for id, batch := range s.batches {
		copy[id] = batch
	}
	return copy
}

func renameInstance(inst LotDescriptor, name string) LotDescriptor {
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
	def, ok := LabelScenarioByKey(key)
	if !ok {
		return nil, fmt.Errorf("unknown label scenario %q", key)
	}
	return solveLabelScenario(def)
}

func solveLabelScenario(def LabelScenarioDefinition) (*LabelScenarioResult, error) {
	label := def.Label
	if label.ServingSize <= 0 && def.Facts.ServingSizeGrams > 0 {
		label = nutritionLabelFromFacts(def.Facts)
	}

	target := label.ToTarget()
	target.POD = Interval{}
	target.PAC = Interval{}
	target.POD = Interval{}
	target.PAC = Interval{}

	lots := def.Lots
	if len(lots) == 0 {
		return nil, fmt.Errorf("label %s missing ingredient lots", def.Name)
	}

	problem := NewFormulationProblem(lots, target)
	if len(def.Batches) > 0 {
		problem.OverrideLots(def.Batches)
	}

	if len(def.Presence) > 0 {
		if err := setPresenceFloor(problem, def.Presence); err != nil {
			return nil, err
		}
	}

	if len(def.Groups) > 0 {
		ApplyGroupBounds(problem, def.Groups)
		ApplyLabelOrder(problem, def.Groups, labelOrderEps())
	} else {
		problem.OrderConstraints = true
	}

	if err := problem.Validate(); err != nil {
		return nil, fmt.Errorf("invalid label problem for %s: %w", def.Name, err)
	}

	pintMass := def.PintMassGrams
	if pintMass == 0 {
		if label.ServingSize > 0 {
			pintMass = label.ServingSize * 3
		} else if def.Facts.ServingSizeGrams > 0 {
			pintMass = def.Facts.ServingSizeGrams * 3
		} else {
			pintMass = 1000
		}
	}

	serveTemp := def.ServeTempC
	if serveTemp == 0 {
		serveTemp = defaultServeTempC
	}
	drawTemp := def.DrawTempC
	if drawTemp == 0 {
		drawTemp = defaultDrawTempC
	}
	shearRate := def.ShearRate
	if shearRate == 0 {
		shearRate = defaultShearRate
	}

	goals := GoalsFromLabel(def.Facts, pintMass, serveTemp, drawTemp, shearRate)
	if def.OverrunCap != nil {
		goals.OverrunCap = def.OverrunCap
	}

	solver, err := NewSolver(problem)
	if err != nil {
		return nil, err
	}
	solution, err := solver.FindSolution()
	if err != nil {
		return nil, fmt.Errorf("%s LP infeasible: %w", def.Name, err)
	}

	specs := def.ScenarioSpecs
	if len(specs) == 0 {
		specs = def.IngredientSpecs
	}
	if len(specs) == 0 {
		specs = problem.Specs()
	}

	recipe, predicted, serving, metrics, err := recipeFromSolution(solution, specs, goals, def.Facts.SodiumMg)
	if err != nil {
		return nil, fmt.Errorf("unable to build recipe for %s: %w", def.Name, err)
	}

	batchDetails := make(map[IngredientID]LotDescriptor, len(solution.Lots))
	for id, lot := range solution.Lots {
		batchDetails[id] = lot
	}

	labelIngredients := def.DisplayNames
	if len(labelIngredients) == 0 {
		for _, spec := range specs {
			labelIngredients = append(labelIngredients, spec.Name)
		}
	}

	return &LabelScenarioResult{
		Name:             def.Name,
		LabelIngredients: labelIngredients,
		LabelFacts:       def.Facts,
		PredictedFacts:   predicted,
		Goals:            goals,
		Problem:          problem,
		Solution:         solution,
		Recipe:           recipe,
		ServingSizeGrams: serving,
		Metrics:          metrics,
		PintMassGrams:    pintMass,
		Specs:            specs,
		BatchDetails:     batchDetails,
	}, nil
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
