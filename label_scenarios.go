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
	Specs            []IngredientSpec
	BatchProfile     BatchProfile
	BatchDetails     map[IngredientID]IngredientLot
}

type scenarioIngredients struct {
	catalog  IngredientCatalog
	batches  map[IngredientID]IngredientLot
	specs    []IngredientSpec
	lots     []IngredientLot
	nameToID map[string]IngredientID
}

func newScenarioIngredients() *scenarioIngredients {
	return &scenarioIngredients{
		catalog:  DefaultIngredientCatalog(),
		batches:  make(map[IngredientID]IngredientLot),
		specs:    make([]IngredientSpec, 0),
		lots:     make([]IngredientLot, 0),
		nameToID: make(map[string]IngredientID),
	}
}

func (s *scenarioIngredients) addClone(key, name string, override func(*IngredientLot)) {
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

func (s *scenarioIngredients) addDetail(inst IngredientLot) {
	profile := inst.EffectiveProfile()
	s.nameToID[profile.Name] = profile.ID
	spec := inst.Ingredient
	spec.Profile = profile
	spec.ID = profile.ID
	spec.Name = profile.Name
	inst = inst.WithSpec(spec)
	inst.Name = profile.Name
	s.batches[profile.ID] = inst
	s.specs = append(s.specs, inst.Ingredient)
	s.lots = append(s.lots, inst)
}

func (s *scenarioIngredients) Specs() []IngredientSpec {
	return s.specs
}

func (s *scenarioIngredients) Lots() []IngredientLot {
	result := make([]IngredientLot, len(s.lots))
	copy(result, s.lots)
	return result
}

func (s *scenarioIngredients) Batches() map[IngredientID]IngredientLot {
	copy := make(map[IngredientID]IngredientLot, len(s.batches))
	for id, batch := range s.batches {
		copy[id] = batch
	}
	return copy
}

func renameInstance(inst IngredientLot, name string) IngredientLot {
	spec := renameSpec(inst.Ingredient, name)
	inst = inst.WithSpec(spec)
	inst.Name = name
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
	builder := newScenarioIngredients()
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("skim_milk", "skim_milk", nil)
	builder.addClone("water", "water", nil)
	builder.addClone("egg_yolk", "egg_yolk", nil)
	builder.addClone("sucrose", "sucrose", nil)
	builder.addClone("guar_gum", "guar_gum", nil)
	builder.addClone("vanilla_extract", "vanilla_extract", nil)
	builder.addClone("vanilla_beans", "vanilla_beans", nil)
	builder.addClone("carrageenan", "carrageenan", nil)
	builder.addClone("sucrose", "liquid_sugar_sucrose", nil)
	builder.addClone("water", "liquid_sugar_water", nil)

	facts := NutritionFacts{
		ServingSizeGrams:  143.0,
		Calories:          330.0,
		TotalFatGrams:     21.3,
		TotalCarbGrams:    28.7,
		TotalSugarsGrams:  28.3,
		ProteinGrams:      5.7,
		SodiumMg:          67.0,
		SaturatedFatGrams: 14.0,
		AddedSugarsGrams:  21.0,
	}
	groups := []LabelGroup{
		{
			Name: "cream",
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "skim_milk", Keys: []IngredientID{builder.id("skim_milk")}},
		{
			Name:                 "liquid_sugar",
			Keys:                 []IngredientID{builder.id("liquid_sugar_sucrose"), builder.id("liquid_sugar_water")},
			EnforceInternalOrder: true,
		},
		{Name: "water", Keys: []IngredientID{builder.id("water")}},
		{Name: "egg_yolk", Keys: []IngredientID{builder.id("egg_yolk")}},
		{Name: "sucrose", Keys: []IngredientID{builder.id("sucrose")}},
		{Name: "guar_gum", Keys: []IngredientID{builder.id("guar_gum")}},
		{Name: "vanilla_extract", Keys: []IngredientID{builder.id("vanilla_extract")}},
		{Name: "vanilla_beans", Keys: []IngredientID{builder.id("vanilla_beans")}},
		{Name: "carrageenan", Keys: []IngredientID{builder.id("carrageenan")}},
	}
	presence := builder.idList("cream_fat", "cream_serum", "skim_milk", "liquid_sugar_sucrose", "liquid_sugar_water", "water", "egg_yolk", "sucrose", "guar_gum", "vanilla_extract", "vanilla_beans", "carrageenan")
	labelIngredients := []string{"cream", "skim milk", "liquid sugar (sucrose, water)", "water", "egg yolks", "sugar", "guar gum", "vanilla extract", "vanilla beans", "carrageenan"}
	return solveLabelScenario("Ben & Jerry's Vanilla", facts, 430.0, builder, groups, presence, labelIngredients)
}

func SolveJenisSweetCream() (*LabelScenarioResult, error) {
	builder := newScenarioIngredients()
	builder.addClone("milk", "milk", nil)
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("sucrose", "cane_sugar", nil)
	builder.addClone("skim_milk", "nonfat_milk", nil)
	builder.addClone("tapioca_syrup", "tapioca_syrup", nil)

	facts := NutritionFacts{
		ServingSizeGrams:  124.0,
		Calories:          316.0,
		TotalFatGrams:     20.0,
		TotalCarbGrams:    28.0,
		TotalSugarsGrams:  23.0,
		ProteinGrams:      6.0,
		SodiumMg:          75.0,
		SaturatedFatGrams: 11.0,
		AddedSugarsGrams:  16.0,
		TransFatGrams:     1.0,
		CholesterolMg:     55.0,
	}
	groups := []LabelGroup{
		{Name: "milk", Keys: []IngredientID{builder.id("milk")}},
		{
			Name: "cream",
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "cane_sugar", Keys: []IngredientID{builder.id("cane_sugar")}},
		{Name: "nonfat_milk", Keys: []IngredientID{builder.id("nonfat_milk")}},
		{Name: "tapioca_syrup", Keys: []IngredientID{builder.id("tapioca_syrup")}},
	}
	presence := builder.idList("milk", "cream_fat", "cream_serum", "cane_sugar", "nonfat_milk", "tapioca_syrup")
	labelIngredients := []string{"milk", "cream", "cane sugar", "nonfat milk", "tapioca syrup"}
	return solveLabelScenario("Jeni's Sweet Cream", facts, facts.ServingSizeGrams*3.0, builder, groups, presence, labelIngredients)
}

func SolveHaagenDazsVanilla() (*LabelScenarioResult, error) {
	builder := newScenarioIngredients()
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("skim_milk", "skim_milk", nil)
	builder.addClone("sucrose", "cane_sugar", nil)
	builder.addClone("egg_yolk", "egg_yolk", nil)
	builder.addClone("vanilla_extract", "vanilla_extract", nil)

	facts := NutritionFacts{
		ServingSizeGrams:  129.0,
		Calories:          320.0,
		TotalFatGrams:     21.0,
		TotalCarbGrams:    26.0,
		TotalSugarsGrams:  25.0,
		ProteinGrams:      6.0,
		SodiumMg:          75.0,
		SaturatedFatGrams: 13.0,
		AddedSugarsGrams:  18.0,
		TransFatGrams:     1.0,
		CholesterolMg:     95.0,
	}
	groups := []LabelGroup{
		{
			Name: "cream",
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "skim_milk", Keys: []IngredientID{builder.id("skim_milk")}},
		{Name: "cane_sugar", Keys: []IngredientID{builder.id("cane_sugar")}},
		{Name: "egg_yolk", Keys: []IngredientID{builder.id("egg_yolk")}},
		{Name: "vanilla_extract", Keys: []IngredientID{builder.id("vanilla_extract")}},
	}
	presence := builder.idList("cream_fat", "cream_serum", "skim_milk", "cane_sugar", "egg_yolk", "vanilla_extract")
	labelIngredients := []string{"cream", "skim milk", "cane sugar", "egg yolks", "vanilla extract"}
	return solveLabelScenario("Haagen-Dazs Vanilla", facts, facts.ServingSizeGrams*3.0, builder, groups, presence, labelIngredients)
}

func SolveBrighamsVanilla() (*LabelScenarioResult, error) {
	builder := newScenarioIngredients()
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("milk", "milk", nil)
	builder.addClone("sucrose", "sugar", nil)
	builder.addClone("vanilla_extract", "vanilla_extract", nil)
	builder.addClone("guar_gum", "guar_gum", nil)
	builder.addClone("salt", "salt", nil)
	builder.addClone("mono_diglycerides", "mono_diglycerides", nil)
	builder.addClone("ps80", "ps80", nil)
	builder.addClone("carrageenan", "carrageenan", nil)
	builder.addClone("potassium_phosphate", "potassium_phosphate", nil)
	builder.addClone("xanthan", "cellulose_gum", nil)

	facts := NutritionFacts{
		ServingSizeGrams:  111.0,
		Calories:          260.0,
		TotalFatGrams:     17.0,
		TotalCarbGrams:    25.0,
		TotalSugarsGrams:  23.0,
		ProteinGrams:      4.0,
		SodiumMg:          95.0,
		SaturatedFatGrams: 10.0,
		AddedSugarsGrams:  17.0,
		TransFatGrams:     0.5,
		CholesterolMg:     65.0,
	}
	groups := []LabelGroup{
		{
			Name: "cream",
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "milk", Keys: []IngredientID{builder.id("milk")}},
		{Name: "sugar", Keys: []IngredientID{builder.id("sugar")}},
		{Name: "vanilla_extract", Keys: []IngredientID{builder.id("vanilla_extract")}},
		{Name: "guar_gum", Keys: []IngredientID{builder.id("guar_gum")}},
		{Name: "salt", Keys: []IngredientID{builder.id("salt")}},
		{Name: "mono_diglycerides", Keys: []IngredientID{builder.id("mono_diglycerides")}},
		{Name: "ps80", Keys: []IngredientID{builder.id("ps80")}},
		{Name: "carrageenan", Keys: []IngredientID{builder.id("carrageenan")}},
		{Name: "potassium_phosphate", Keys: []IngredientID{builder.id("potassium_phosphate")}},
		{Name: "cellulose_gum", Keys: []IngredientID{builder.id("cellulose_gum")}},
	}
	presence := builder.idList("cream_fat", "cream_serum", "milk", "sugar", "vanilla_extract", "guar_gum", "salt", "mono_diglycerides", "ps80", "carrageenan", "potassium_phosphate", "cellulose_gum")
	labelIngredients := []string{"cream", "milk", "sugar", "vanilla extract", "guar gum", "salt", "mono & diglycerides", "ps80", "carrageenan", "potassium phosphate", "cellulose gum"}
	return solveLabelScenario("Brigham's Vanilla", facts, facts.ServingSizeGrams*3.0, builder, groups, presence, labelIngredients)
}

func SolveBreyersVanilla() (*LabelScenarioResult, error) {
	builder := newScenarioIngredients()
	builder.addClone("milk", "milk", nil)
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("sucrose", "sugar", nil)
	builder.addClone("skim_milk", "skim_milk", nil)
	builder.addClone("tara_gum", "tara_gum", nil)
	builder.addClone("vanilla_extract", "natural_flavor", nil)

	facts := NutritionFacts{
		ServingSizeGrams:  88.0,
		Calories:          170.0,
		TotalFatGrams:     9.0,
		TotalCarbGrams:    19.0,
		TotalSugarsGrams:  19.0,
		ProteinGrams:      3.0,
		SodiumMg:          50.0,
		SaturatedFatGrams: 6.0,
		AddedSugarsGrams:  14.0,
		CholesterolMg:     25.0,
	}
	groups := []LabelGroup{
		{Name: "milk", Keys: []IngredientID{builder.id("milk")}},
		{
			Name: "cream",
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "sugar", Keys: []IngredientID{builder.id("sugar")}},
		{Name: "skim_milk", Keys: []IngredientID{builder.id("skim_milk")}},
		{Name: "tara_gum", Keys: []IngredientID{builder.id("tara_gum")}},
		{Name: "natural_flavor", Keys: []IngredientID{builder.id("natural_flavor")}},
	}
	presence := builder.idList("milk", "cream_fat", "cream_serum", "sugar", "skim_milk", "tara_gum", "natural_flavor")
	labelIngredients := []string{"milk", "cream", "sugar", "skim milk", "tara gum", "natural flavor"}
	return solveLabelScenario("Breyers Vanilla", facts, facts.ServingSizeGrams*3.0, builder, groups, presence, labelIngredients)
}

func SolveTalentiVanilla() (*LabelScenarioResult, error) {
	builder := newScenarioIngredients()
	builder.addClone("milk", "milk", nil)
	builder.addClone("sucrose", "sugar", nil)
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("dextrose", "dextrose", nil)
	builder.addClone("vanilla_extract", "vanilla_extract", nil)
	builder.addClone("lecithin", "sunflower_lecithin", nil)
	builder.addClone("locust_bean_gum", "carob_bean_gum", nil)
	builder.addClone("guar_gum", "guar_gum", nil)
	builder.addClone("vanilla_extract", "natural_flavor", func(inst *IngredientLot) {
		profile := inst.Ingredient.Profile
		profile.Components.Water = Point(0.60)
		profile.Components.OtherSolids = Point(0.40)
		inst.Ingredient.Profile = profile
	})
	builder.addClone("lemon_peel", "lemon_peel", nil)

	facts := NutritionFacts{
		ServingSizeGrams:  128.0,
		Calories:          260.0,
		TotalFatGrams:     13.0,
		TotalCarbGrams:    31.0,
		TotalSugarsGrams:  30.0,
		ProteinGrams:      5.0,
		SodiumMg:          70.0,
		SaturatedFatGrams: 8.0,
		AddedSugarsGrams:  22.0,
		CholesterolMg:     45.0,
	}
	groups := []LabelGroup{
		{Name: "milk", Keys: []IngredientID{builder.id("milk")}},
		{Name: "sugar", Keys: []IngredientID{builder.id("sugar")}},
		{
			Name: "cream",
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "dextrose", Keys: []IngredientID{builder.id("dextrose")}},
		{Name: "vanilla_extract", Keys: []IngredientID{builder.id("vanilla_extract")}},
		{Name: "sunflower_lecithin", Keys: []IngredientID{builder.id("sunflower_lecithin")}},
		{Name: "carob_bean_gum", Keys: []IngredientID{builder.id("carob_bean_gum")}},
		{Name: "guar_gum", Keys: []IngredientID{builder.id("guar_gum")}},
		{Name: "natural_flavor", Keys: []IngredientID{builder.id("natural_flavor")}},
		{Name: "lemon_peel", Keys: []IngredientID{builder.id("lemon_peel")}},
	}
	presence := builder.idList("milk", "sugar", "cream_fat", "cream_serum", "dextrose", "vanilla_extract", "sunflower_lecithin", "carob_bean_gum", "guar_gum", "natural_flavor", "lemon_peel")
	labelIngredients := []string{"milk", "sugar", "cream", "dextrose", "vanilla extract", "sunflower lecithin", "carob bean gum", "guar gum", "natural flavor", "lemon peel"}
	return solveLabelScenario("Talenti Vanilla Bean", facts, facts.ServingSizeGrams*3.0, builder, groups, presence, labelIngredients)
}

func solveLabelScenario(name string, facts NutritionFacts, pintMass float64, builder *scenarioIngredients, groups []LabelGroup, presence []IngredientID, labelIngredients []string) (*LabelScenarioResult, error) {
	goals := GoalsFromLabel(facts, pintMass, defaultServeTempC, defaultDrawTempC, defaultShearRate)
	target := scenarioTargetFromFacts(facts)

	problem := NewFormulationProblem(builder.Lots(), target)
	problem.OverrideLots(builder.Batches())
	specs := builder.Specs()

	if err := setPresenceFloor(problem, presence); err != nil {
		return nil, err
	}

	ApplyGroupBounds(problem, groups)
	ApplyLabelOrder(problem, groups, labelOrderEps())

	if err := problem.Validate(); err != nil {
		return nil, fmt.Errorf("invalid label problem for %s: %w", name, err)
	}

	solver, err := NewSolver(problem)
	if err != nil {
		return nil, err
	}
	solution, err := solver.FindSolution()
	// Fallback: try to find feasible solution with random objective? but for now return error.
	if err != nil {
		return nil, fmt.Errorf("%s LP infeasible: %w", name, err)
	}

	recipe, predicted, serving, metrics, err := recipeFromSolution(solution, specs, goals, facts.SodiumMg)
	if err != nil {
		return nil, fmt.Errorf("unable to build recipe for %s: %w", name, err)
	}

	batchProfile := BuildBatchProfile(solution.Weights, specs, solution.Lots)
	batchDetails := make(map[IngredientID]IngredientLot, len(solution.Lots))
	for id, lot := range solution.Lots {
		batchDetails[id] = lot
	}

	return &LabelScenarioResult{
		Name:             name,
		LabelIngredients: labelIngredients,
		LabelFacts:       facts,
		PredictedFacts:   predicted,
		Goals:            goals,
		Problem:          problem,
		Solution:         solution,
		Recipe:           recipe,
		ServingSizeGrams: serving,
		Metrics:          metrics,
		PintMassGrams:    pintMass,
		Specs:            specs,
		BatchProfile:     batchProfile,
		BatchDetails:     batchDetails,
	}, nil
}

func scenarioTargetFromFacts(facts NutritionFacts) FormulationTarget {
	label := NutritionLabel{
		ServingSize: facts.ServingSizeGrams,
		Calories:    facts.Calories,
		TotalFat:    facts.TotalFatGrams,
		TotalCarbs:  facts.TotalCarbGrams,
		Sugars:      facts.TotalSugarsGrams,
		AddedSugars: facts.AddedSugarsGrams,
		Protein:     facts.ProteinGrams,
	}
	return label.ToTarget()
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
