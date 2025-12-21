package creamery

import (
	"errors"
	"fmt"
)

type NutritionFacts struct {
	ServingSizeGrams   float64
	Calories           float64
	TotalFatGrams      float64
	TotalCarbGrams     float64
	TotalSugarsGrams   float64
	ProteinGrams       float64
	SodiumMg           float64
	SaturatedFatGrams  float64
	SaturatedFatPct    float64
	TransFatGrams      float64
	TransFatPct        float64
	AddedSugarsGrams   float64
	AddedSugarsPct     float64
	FatPct             float64
	CarbsPct           float64
	SugarsPct          float64
	ProteinPct         float64
	CholesterolMg      float64
	CholesterolMgPerKg float64
}

// Formulation aggregates mix composition into batch fractions.
type Formulation struct {
	MilkfatPct    float64
	SNFPct        float64
	WaterPct      float64
	SugarsPct     map[string]float64
	StabilizerPct float64
	EmulsifierPct float64
	ProteinPct    float64
}

// ProductionSettings captures process conditions and derived metrics.
type ProductionSettings struct {
	ServeTempC float64
	DrawTempC  float64
	ShearRate  float64
	OverrunCap *float64
	Metrics    map[string]float64
}

// RecipeComponent couples an ingredient with a batch weight (kg).
type RecipeComponent struct {
	Ingredient *IngredientBatch
	MassKg     float64
}

// Recipe contains concrete ingredient weights and optional metadata.
type Recipe struct {
	Components  []RecipeComponent
	Overrun     float64
	Notes       []string
	MixSnapshot *ProductionSettings
}

// NewRecipe validates and constructs a recipe from components.
func NewRecipe(components []RecipeComponent, overrun float64) (*Recipe, error) {
	if overrun < 0 {
		return nil, errors.New("overrun cannot be negative")
	}
	if len(components) == 0 {
		return nil, errors.New("recipe must contain at least one component")
	}
	clean := make([]RecipeComponent, 0, len(components))
	for _, comp := range components {
		if comp.Ingredient == nil {
			return nil, errors.New("component ingredient cannot be nil")
		}
		if comp.MassKg < 0 {
			return nil, fmt.Errorf("ingredient %s weight cannot be negative", comp.Ingredient.Name)
		}
		if comp.MassKg == 0 {
			continue
		}
		clean = append(clean, comp)
	}
	if len(clean) == 0 {
		return nil, errors.New("recipe has zero total mass")
	}
	return &Recipe{
		Components: clean,
		Overrun:    overrun,
	}, nil
}

func NewRecipeFromWeights(ingredients []*IngredientBatch, weights []float64, overrun float64) (*Recipe, error) {
	if len(ingredients) != len(weights) {
		return nil, errors.New("ingredient and weight slices must have equal length")
	}
	components := make([]RecipeComponent, 0, len(ingredients))
	for i, ing := range ingredients {
		if ing == nil {
			return nil, errors.New("ingredient cannot be nil")
		}
		mass := weights[i]
		if mass <= 0 {
			continue
		}
		components = append(components, RecipeComponent{
			Ingredient: ing,
			MassKg:     mass,
		})
	}
	return NewRecipe(components, overrun)
}

// WithOverrun returns a shallow copy with updated overrun.
func (r Recipe) WithOverrun(overrun float64) (Recipe, error) {
	if overrun < 0 {
		return Recipe{}, errors.New("overrun cannot be negative")
	}
	r.Overrun = overrun
	return r, nil
}

// WithNotes returns a shallow copy with updated notes.
func (r Recipe) WithNotes(notes []string) Recipe {
	cpy := make([]string, len(notes))
	copy(cpy, notes)
	r.Notes = cpy
	return r
}

// WithMixSnapshot stores process settings/metrics.
func (r Recipe) WithMixSnapshot(snapshot *ProductionSettings) Recipe {
	r.MixSnapshot = snapshot
	return r
}

// BatchMassKg returns the total batch mass.
func (r *Recipe) BatchMassKg() float64 {
	total := 0.0
	for _, comp := range r.Components {
		total += comp.MassKg
	}
	return total
}

// Fractions returns ingredient mass fractions keyed by name.
func (r *Recipe) Fractions() map[string]float64 {
	total := r.BatchMassKg()
	if total <= 0 {
		return map[string]float64{}
	}
	fractions := make(map[string]float64, len(r.Components))
	for _, comp := range r.Components {
		if comp.MassKg <= 0 {
			continue
		}
		name := comp.Ingredient.Name
		fractions[name] += comp.MassKg / total
	}
	return fractions
}

func (r *Recipe) namedWeights() ([]string, []float64, map[string]IngredientBatch) {
	index := make(map[string]int)
	keys := make([]string, 0, len(r.Components))
	weights := make([]float64, 0, len(r.Components))
	table := make(map[string]IngredientBatch, len(r.Components))

	for _, comp := range r.Components {
		if comp.MassKg <= 0 || comp.Ingredient == nil {
			continue
		}
		name := comp.Ingredient.Name
		if idx, ok := index[name]; ok {
			weights[idx] += comp.MassKg
		} else {
			index[name] = len(keys)
			keys = append(keys, name)
			weights = append(weights, comp.MassKg)
		}
		table[name] = *comp.Ingredient
	}
	return keys, weights, table
}

func (r *Recipe) ComponentTotals() map[string]float64 {
	keys, weights, table := r.namedWeights()
	return componentSums(keys, weights, table)
}

// Formulation summarizes composition into the Formulation struct.
func (r *Recipe) Formulation() (Formulation, error) {
	totals := r.ComponentTotals()
	batch := totals["total"]
	if batch <= 0 {
		return Formulation{}, errors.New("recipe has zero total mass")
	}
	fat := totals["fat"] / batch
	protein := totals["protein"] / batch
	water := totals["water"] / batch
	snf := (totals["protein"] + totals["lactose"] + totals["ash"]) / batch
	sugars := map[string]float64{
		"sucrose":      totals["sucrose"] / batch,
		"glucose":      totals["glucose"] / batch,
		"fructose":     totals["fructose"] / batch,
		"lactose":      totals["lactose"] / batch,
		"polyols":      totals["polyols"] / batch,
		"maltodextrin": totals["maltodextrin"] / batch,
	}

	stabilizer := 0.0
	emulsifier := 0.0
	for _, comp := range r.Components {
		if comp.Ingredient == nil || comp.MassKg <= 0 {
			continue
		}
		if comp.Ingredient.Hydrocolloid {
			stabilizer += comp.MassKg * (comp.Ingredient.OtherSolids + comp.Ingredient.Maltodextrin + comp.Ingredient.Polyols)
		}
		if comp.Ingredient.EmulsifierPower > 0 {
			emulsifier += comp.MassKg
		}
	}

	return Formulation{
		MilkfatPct:    fat,
		SNFPct:        snf,
		WaterPct:      water,
		SugarsPct:     sugars,
		StabilizerPct: stabilizer / batch,
		EmulsifierPct: emulsifier / batch,
		ProteinPct:    protein,
	}, nil
}

// SweetnessPct returns the sucrose-equivalent solids percentage.
func (r *Recipe) SweetnessPct() float64 {
	totals := r.ComponentTotals()
	totalMass := totals["total"]
	if totalMass <= 0 {
		return 0
	}
	return sweetnessEq(totals) / totalMass
}

// CostPerKg returns batch cost per kilogram.
func (r *Recipe) CostPerKg() float64 {
	total := r.BatchMassKg()
	if total <= 0 {
		return 0
	}
	sum := 0.0
	for _, comp := range r.Components {
		if comp.Ingredient == nil {
			continue
		}
		sum += comp.MassKg * comp.Ingredient.Cost
	}
	return sum / total
}

func (r *Recipe) mixMetrics(opts MixOptions) (map[string]float64, error) {
	keys, weights, table := r.namedWeights()
	if len(keys) == 0 {
		return nil, errors.New("recipe has no mass")
	}
	return BuildProperties(keys, weights, table, opts), nil
}

// FreezingPoint calculates the freezing point (°C).
func (r *Recipe) FreezingPoint(opts MixOptions) (float64, error) {
	metrics, err := r.mixMetrics(opts)
	if err != nil {
		return 0, err
	}
	return metrics["freezing_point"], nil
}

// OverrunCeiling returns the predicted overrun limit.
func (r *Recipe) OverrunCeiling(opts MixOptions) (float64, error) {
	metrics, err := r.mixMetrics(opts)
	if err != nil {
		return 0, err
	}
	return metrics["overrun_estimate"], nil
}

// MixVolumeL returns pre-overrun batch volume in liters.
func (r *Recipe) MixVolumeL(opts MixOptions) (float64, error) {
	metrics, err := r.mixMetrics(opts)
	if err != nil {
		return 0, err
	}
	return metrics["volume_L"], nil
}

// ServingSizeForVolume converts a draw volume to serving grams.
func (r *Recipe) ServingSizeForVolume(portionL float64, opts MixOptions) (float64, error) {
	totals := r.ComponentTotals()
	mixVolume, err := r.MixVolumeL(opts)
	if err != nil {
		return 0, err
	}
	if mixVolume <= 0 {
		return 0, errors.New("mix volume is zero")
	}
	density := totals["total"] / (mixVolume * (1.0 + r.Overrun))
	return density * portionL * 1000.0, nil
}

// NutritionFacts computes per-serving nutrition.
func (r *Recipe) NutritionFacts(servingSizeGrams float64, sodiumMg float64) (NutritionFacts, error) {
	totals := r.ComponentTotals()
	batch := totals["total"]
	if batch <= 0 {
		return NutritionFacts{}, errors.New("recipe has zero total mass")
	}
	fatPct := totals["fat"] / batch
	sugarsPct := (totals["sucrose"] + totals["glucose"] + totals["fructose"] + totals["lactose"]) / batch
	proteinPct := totals["protein"] / batch
	snfPct := (totals["protein"] + totals["lactose"] + totals["ash"]) / batch
	carbsPct := sugarsPct + snfPct - proteinPct
	transFatPct := totals["trans_fat"] / batch
	saturatedFatPct := totals["saturated_fat"] / batch
	addedSugarsPct := totals["added_sugars"] / batch

	fatG := fatPct * servingSizeGrams
	carbsG := carbsPct * servingSizeGrams
	proteinG := proteinPct * servingSizeGrams
	sugarsG := sugarsPct * servingSizeGrams
	transFatG := transFatPct * servingSizeGrams
	satFatG := saturatedFatPct * servingSizeGrams
	addedSugarsG := addedSugarsPct * servingSizeGrams
	cholMgPerKg := 0.0
	if batch > 0 {
		cholMgPerKg = totals["cholesterol_mg"] / batch
	}
	cholMg := cholMgPerKg * (servingSizeGrams / 1000.0)
	calories := 9*fatG + 4*carbsG + 4*proteinG

	return NutritionFacts{
		ServingSizeGrams:   servingSizeGrams,
		Calories:           calories,
		TotalFatGrams:      fatG,
		TotalCarbGrams:     carbsG,
		TotalSugarsGrams:   sugarsG,
		ProteinGrams:       proteinG,
		SodiumMg:           sodiumMg,
		SaturatedFatGrams:  satFatG,
		SaturatedFatPct:    saturatedFatPct,
		TransFatGrams:      transFatG,
		TransFatPct:        transFatPct,
		AddedSugarsGrams:   addedSugarsG,
		AddedSugarsPct:     addedSugarsPct,
		FatPct:             fatPct,
		CarbsPct:           carbsPct,
		SugarsPct:          sugarsPct,
		ProteinPct:         proteinPct,
		CholesterolMg:      cholMg,
		CholesterolMgPerKg: cholMgPerKg,
	}, nil
}
