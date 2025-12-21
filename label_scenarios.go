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
	Metrics          map[string]float64
	PintMassGrams    float64
}

type scenarioIngredients struct {
	table       map[string]IngredientBatch
	batches     map[string]IngredientBatch
	specs       []IngredientSpec
}

func newScenarioIngredients() *scenarioIngredients {
	return &scenarioIngredients{
		table:       IngredientBatchTable(),
		batches:     make(map[string]IngredientBatch),
		specs:       make([]IngredientSpec, 0),
	}
}

func (s *scenarioIngredients) addClone(key, name string, override func(*IngredientBatch), sweetener SweetenerProps) {
	base := s.table[key]
	detail := cloneBatch(base, func(d *IngredientBatch) {
		d.Name = name
		if override != nil {
			override(d)
		}
	})
	s.addDetail(detail, sweetener)
}

func (s *scenarioIngredients) addCustom(detail IngredientBatch, sweetener SweetenerProps) {
	s.addDetail(detail, sweetener)
}

func (s *scenarioIngredients) addDetail(detail IngredientBatch, sweetener SweetenerProps) {
	s.batches[detail.Name] = detail
	s.specs = append(s.specs, ingredientSpecFromBatch(detail, sweetener))
}

func (s *scenarioIngredients) Specs() []IngredientSpec {
	return s.specs
}

func (s *scenarioIngredients) LegacyIngredients() []Ingredient {
	legacy := make([]Ingredient, len(s.specs))
	for i, spec := range s.specs {
		legacy[i] = spec.LegacyIngredient()
	}
	return legacy
}

func (s *scenarioIngredients) Batches() map[string]IngredientBatch {
	return s.batches
}

// SolveBenAndJerryVanilla recreates the Ben & Jerry's Vanilla label problem.
func SolveBenAndJerryVanilla() (*LabelScenarioResult, error) {
	builder := newScenarioIngredients()
	builder.addClone("cream_fat", "cream_fat", nil, SweetenerProps{})
	builder.addClone("cream_serum", "cream_serum", nil, SweetenerProps{})
	builder.addClone("skim_milk", "skim_milk", nil, SweetenerProps{})
	builder.addClone("water", "water", nil, SweetenerProps{})
	builder.addClone("egg_yolk", "egg_yolk", nil, SweetenerProps{})
	builder.addClone("sucrose", "sucrose", nil, Sucrose)
	builder.addClone("guar_gum", "guar_gum", nil, SweetenerProps{})
	builder.addClone("vanilla_extract", "vanilla_extract", nil, SweetenerProps{})
	builder.addClone("vanilla_beans", "vanilla_beans", nil, SweetenerProps{})
	builder.addClone("carrageenan", "carrageenan", nil, SweetenerProps{})
	builder.addClone("sucrose", "liquid_sugar_sucrose", nil, Sucrose)
	builder.addClone("water", "liquid_sugar_water", nil, SweetenerProps{})

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
			Keys: []string{"cream_fat", "cream_serum"},
			FractionBounds: map[string]Interval{
				"cream_fat": RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "skim_milk", Keys: []string{"skim_milk"}},
		{
			Name:                 "liquid_sugar",
			Keys:                 []string{"liquid_sugar_sucrose", "liquid_sugar_water"},
			EnforceInternalOrder: true,
		},
		{Name: "water", Keys: []string{"water"}},
		{Name: "egg_yolk", Keys: []string{"egg_yolk"}},
		{Name: "sucrose", Keys: []string{"sucrose"}},
		{Name: "guar_gum", Keys: []string{"guar_gum"}},
		{Name: "vanilla_extract", Keys: []string{"vanilla_extract"}},
		{Name: "vanilla_beans", Keys: []string{"vanilla_beans"}},
		{Name: "carrageenan", Keys: []string{"carrageenan"}},
	}
	presence := []string{"cream_fat", "cream_serum", "skim_milk", "liquid_sugar_sucrose", "liquid_sugar_water", "water", "egg_yolk", "sucrose", "guar_gum", "vanilla_extract", "vanilla_beans", "carrageenan"}
	labelIngredients := []string{"cream", "skim milk", "liquid sugar (sucrose, water)", "water", "egg yolks", "sugar", "guar gum", "vanilla extract", "vanilla beans", "carrageenan"}
	return solveLabelScenario("Ben & Jerry's Vanilla", facts, 430.0, builder, groups, presence, labelIngredients)
}

func SolveJenisSweetCream() (*LabelScenarioResult, error) {
	builder := newScenarioIngredients()
	builder.addClone("milk", "milk", nil, SweetenerProps{})
	builder.addClone("cream_fat", "cream_fat", nil, SweetenerProps{})
	builder.addClone("cream_serum", "cream_serum", nil, SweetenerProps{})
	builder.addClone("sucrose", "cane_sugar", nil, Sucrose)
	builder.addClone("skim_milk", "nonfat_milk", nil, SweetenerProps{})
	builder.addClone("tapioca_syrup", "tapioca_syrup", nil, TapiocaSyrupS)

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
		{Name: "milk", Keys: []string{"milk"}},
		{
			Name: "cream",
			Keys: []string{"cream_fat", "cream_serum"},
			FractionBounds: map[string]Interval{
				"cream_fat": RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "cane_sugar", Keys: []string{"cane_sugar"}},
		{Name: "nonfat_milk", Keys: []string{"nonfat_milk"}},
		{Name: "tapioca_syrup", Keys: []string{"tapioca_syrup"}},
	}
	presence := []string{"milk", "cream_fat", "cream_serum", "cane_sugar", "nonfat_milk", "tapioca_syrup"}
	labelIngredients := []string{"milk", "cream", "cane sugar", "nonfat milk", "tapioca syrup"}
	return solveLabelScenario("Jeni's Sweet Cream", facts, facts.ServingSizeGrams*3.0, builder, groups, presence, labelIngredients)
}

func SolveHaagenDazsVanilla() (*LabelScenarioResult, error) {
	builder := newScenarioIngredients()
	builder.addClone("cream_fat", "cream_fat", nil, SweetenerProps{})
	builder.addClone("cream_serum", "cream_serum", nil, SweetenerProps{})
	builder.addClone("skim_milk", "skim_milk", nil, SweetenerProps{})
	builder.addClone("sucrose", "cane_sugar", nil, Sucrose)
	builder.addClone("egg_yolk", "egg_yolk", nil, SweetenerProps{})
	builder.addClone("vanilla_extract", "vanilla_extract", nil, SweetenerProps{})

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
			Keys: []string{"cream_fat", "cream_serum"},
			FractionBounds: map[string]Interval{
				"cream_fat": RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "skim_milk", Keys: []string{"skim_milk"}},
		{Name: "cane_sugar", Keys: []string{"cane_sugar"}},
		{Name: "egg_yolk", Keys: []string{"egg_yolk"}},
		{Name: "vanilla_extract", Keys: []string{"vanilla_extract"}},
	}
	presence := []string{"cream_fat", "cream_serum", "skim_milk", "cane_sugar", "egg_yolk", "vanilla_extract"}
	labelIngredients := []string{"cream", "skim milk", "cane sugar", "egg yolks", "vanilla extract"}
	return solveLabelScenario("Haagen-Dazs Vanilla", facts, facts.ServingSizeGrams*3.0, builder, groups, presence, labelIngredients)
}

func SolveBrighamsVanilla() (*LabelScenarioResult, error) {
	builder := newScenarioIngredients()
	builder.addClone("cream_fat", "cream_fat", nil, SweetenerProps{})
	builder.addClone("cream_serum", "cream_serum", nil, SweetenerProps{})
	builder.addClone("milk", "milk", nil, SweetenerProps{})
	builder.addClone("sucrose", "sugar", nil, Sucrose)
	builder.addClone("vanilla_extract", "vanilla_extract", nil, SweetenerProps{})
	builder.addClone("guar_gum", "guar_gum", nil, SweetenerProps{})
	builder.addClone("salt", "salt", nil, SweetenerProps{})
	builder.addClone("mono_diglycerides", "mono_diglycerides", nil, SweetenerProps{})
	builder.addClone("ps80", "ps80", nil, SweetenerProps{})
	builder.addClone("carrageenan", "carrageenan", nil, SweetenerProps{})
	builder.addClone("potassium_phosphate", "potassium_phosphate", nil, SweetenerProps{})
	builder.addClone("xanthan", "cellulose_gum", nil, SweetenerProps{})

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
			Keys: []string{"cream_fat", "cream_serum"},
			FractionBounds: map[string]Interval{
				"cream_fat": RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "milk", Keys: []string{"milk"}},
		{Name: "sugar", Keys: []string{"sugar"}},
		{Name: "vanilla_extract", Keys: []string{"vanilla_extract"}},
		{Name: "guar_gum", Keys: []string{"guar_gum"}},
		{Name: "salt", Keys: []string{"salt"}},
		{Name: "mono_diglycerides", Keys: []string{"mono_diglycerides"}},
		{Name: "ps80", Keys: []string{"ps80"}},
		{Name: "carrageenan", Keys: []string{"carrageenan"}},
		{Name: "potassium_phosphate", Keys: []string{"potassium_phosphate"}},
		{Name: "cellulose_gum", Keys: []string{"cellulose_gum"}},
	}
	presence := []string{"cream_fat", "cream_serum", "milk", "sugar", "vanilla_extract", "guar_gum", "salt", "mono_diglycerides", "ps80", "carrageenan", "potassium_phosphate", "cellulose_gum"}
	labelIngredients := []string{"cream", "milk", "sugar", "vanilla extract", "guar gum", "salt", "mono & diglycerides", "ps80", "carrageenan", "potassium phosphate", "cellulose gum"}
	return solveLabelScenario("Brigham's Vanilla", facts, facts.ServingSizeGrams*3.0, builder, groups, presence, labelIngredients)
}

func SolveBreyersVanilla() (*LabelScenarioResult, error) {
	builder := newScenarioIngredients()
	builder.addClone("milk", "milk", nil, SweetenerProps{})
	builder.addClone("cream_fat", "cream_fat", nil, SweetenerProps{})
	builder.addClone("cream_serum", "cream_serum", nil, SweetenerProps{})
	builder.addClone("sucrose", "sugar", nil, Sucrose)
	builder.addClone("skim_milk", "skim_milk", nil, SweetenerProps{})
	builder.addClone("tara_gum", "tara_gum", nil, SweetenerProps{})
	builder.addClone("vanilla_extract", "natural_flavor", nil, SweetenerProps{})

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
		{Name: "milk", Keys: []string{"milk"}},
		{
			Name: "cream",
			Keys: []string{"cream_fat", "cream_serum"},
			FractionBounds: map[string]Interval{
				"cream_fat": RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "sugar", Keys: []string{"sugar"}},
		{Name: "skim_milk", Keys: []string{"skim_milk"}},
		{Name: "tara_gum", Keys: []string{"tara_gum"}},
		{Name: "natural_flavor", Keys: []string{"natural_flavor"}},
	}
	presence := []string{"milk", "cream_fat", "cream_serum", "sugar", "skim_milk", "tara_gum", "natural_flavor"}
	labelIngredients := []string{"milk", "cream", "sugar", "skim milk", "tara gum", "natural flavor"}
	return solveLabelScenario("Breyers Vanilla", facts, facts.ServingSizeGrams*3.0, builder, groups, presence, labelIngredients)
}

func SolveTalentiVanilla() (*LabelScenarioResult, error) {
	builder := newScenarioIngredients()
	builder.addClone("milk", "milk", nil, SweetenerProps{})
	builder.addClone("sucrose", "sugar", nil, Sucrose)
	builder.addClone("cream_fat", "cream_fat", nil, SweetenerProps{})
	builder.addClone("cream_serum", "cream_serum", nil, SweetenerProps{})
	builder.addClone("dextrose", "dextrose", nil, Dextrose)
	builder.addClone("vanilla_extract", "vanilla_extract", nil, SweetenerProps{})
	builder.addClone("lecithin", "sunflower_lecithin", nil, SweetenerProps{})
	builder.addClone("locust_bean_gum", "carob_bean_gum", nil, SweetenerProps{})
	builder.addClone("guar_gum", "guar_gum", nil, SweetenerProps{})
	builder.addClone("vanilla_extract", "natural_flavor", func(d *IngredientBatch) {
		d.Water = 0.60
		d.OtherSolids = 0.40
	}, SweetenerProps{})
	builder.addClone("lemon_peel", "lemon_peel", nil, SweetenerProps{})

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
		{Name: "milk", Keys: []string{"milk"}},
		{Name: "sugar", Keys: []string{"sugar"}},
		{
			Name: "cream",
			Keys: []string{"cream_fat", "cream_serum"},
			FractionBounds: map[string]Interval{
				"cream_fat": RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "dextrose", Keys: []string{"dextrose"}},
		{Name: "vanilla_extract", Keys: []string{"vanilla_extract"}},
		{Name: "sunflower_lecithin", Keys: []string{"sunflower_lecithin"}},
		{Name: "carob_bean_gum", Keys: []string{"carob_bean_gum"}},
		{Name: "guar_gum", Keys: []string{"guar_gum"}},
		{Name: "natural_flavor", Keys: []string{"natural_flavor"}},
		{Name: "lemon_peel", Keys: []string{"lemon_peel"}},
	}
	presence := []string{"milk", "sugar", "cream_fat", "cream_serum", "dextrose", "vanilla_extract", "sunflower_lecithin", "carob_bean_gum", "guar_gum", "natural_flavor", "lemon_peel"}
	labelIngredients := []string{"milk", "sugar", "cream", "dextrose", "vanilla extract", "sunflower lecithin", "carob bean gum", "guar gum", "natural flavor", "lemon peel"}
	return solveLabelScenario("Talenti Vanilla Bean", facts, facts.ServingSizeGrams*3.0, builder, groups, presence, labelIngredients)
}

func solveLabelScenario(name string, facts NutritionFacts, pintMass float64, builder *scenarioIngredients, groups []LabelGroup, presence []string, labelIngredients []string) (*LabelScenarioResult, error) {
	goals := GoalsFromLabel(facts, pintMass, defaultServeTempC, defaultDrawTempC, defaultShearRate)
	target := scenarioTargetFromFacts(facts)

	problem := &Problem{
		Ingredients:      builder.LegacyIngredients(),
		Target:           target,
		WeightBounds:     make(map[string]Interval),
		OrderConstraints: false,
	}

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

	recipe, predicted, serving, metrics, err := recipeFromSolution(solution, builder.Specs(), builder.Batches(), goals, facts.SodiumMg)
	if err != nil {
		return nil, fmt.Errorf("unable to build recipe for %s: %w", name, err)
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
	}, nil
}

func ingredientSpecFromBatch(detail IngredientBatch, sweetener SweetenerProps) IngredientSpec {
	return SpecFromProfile(detail.ToProfile(), sweetener)
}

func scenarioTargetFromFacts(facts NutritionFacts) Composition {
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

func setPresenceFloor(p *Problem, names []string) error {
	for _, name := range names {
		if err := p.SetMinWeight(name, presenceFloorFraction); err != nil {
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
