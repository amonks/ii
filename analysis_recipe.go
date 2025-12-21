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
	Components    ConstituentComponents
}

// ProductionSettings captures process conditions and derived metrics.
type ProductionSettings struct {
	ServeTempC float64
	DrawTempC  float64
	ShearRate  float64
	OverrunCap *float64
	Snapshot   BatchSnapshot
}

// RecipeComponent couples an ingredient lot with a batch weight (kg).
type RecipeComponent struct {
	Ingredient IngredientLot
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
		if comp.Ingredient.Ingredient.ID == "" && comp.Ingredient.DisplayName() == "" {
			return nil, errors.New("component ingredient cannot be empty")
		}
		if comp.MassKg < 0 {
			return nil, fmt.Errorf("ingredient %s weight cannot be negative", comp.Ingredient.DisplayName())
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

func NewRecipeFromWeights(ingredients []IngredientLot, weights []float64, overrun float64) (*Recipe, error) {
	if len(ingredients) != len(weights) {
		return nil, errors.New("ingredient and weight slices must have equal length")
	}
	components := make([]RecipeComponent, 0, len(ingredients))
	for i, ing := range ingredients {
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
		name := comp.Ingredient.DisplayName()
		fractions[name] += comp.MassKg / total
	}
	return fractions
}

func (r *Recipe) aggregateTotals() (BatchSnapshot, error) {
	return NewBatchSnapshot(r.Components)
}

func (r *Recipe) mixSnapshot(opts MixOptions) (BatchSnapshot, error) {
	return BuildProperties(r.Components, opts)
}

// Formulation summarizes composition into the Formulation struct.
func (r *Recipe) Formulation() (Formulation, error) {
	snapshot, err := r.aggregateTotals()
	if err != nil {
		return Formulation{}, err
	}
	return snapshot.FormulationBreakdown()
}

// SweetnessPct returns the sucrose-equivalent solids percentage.
func (r *Recipe) SweetnessPct() float64 {
	snapshot, err := r.aggregateTotals()
	if err != nil || snapshot.TotalMassKg <= 0 {
		return 0
	}
	return snapshot.SweetnessEq / snapshot.TotalMassKg
}

// CostPerKg returns batch cost per kilogram.
func (r *Recipe) CostPerKg() float64 {
	snapshot, err := r.aggregateTotals()
	if err != nil || snapshot.TotalMassKg <= 0 {
		return 0
	}
	return snapshot.CostPerKg
}

func (r *Recipe) mixSnapshotWithOptions(opts MixOptions) (BatchSnapshot, error) {
	return BuildProperties(r.Components, opts)
}

// FreezingPoint calculates the freezing point (°C).
func (r *Recipe) FreezingPoint(opts MixOptions) (float64, error) {
	snapshot, err := r.mixSnapshotWithOptions(opts)
	if err != nil {
		return 0, err
	}
	return snapshot.FreezingPointC, nil
}

// OverrunCeiling returns the predicted overrun limit.
func (r *Recipe) OverrunCeiling(opts MixOptions) (float64, error) {
	snapshot, err := r.mixSnapshotWithOptions(opts)
	if err != nil {
		return 0, err
	}
	return snapshot.OverrunEstimate, nil
}

// MixVolumeL returns pre-overrun batch volume in liters.
func (r *Recipe) MixVolumeL(opts MixOptions) (float64, error) {
	snapshot, err := r.mixSnapshotWithOptions(opts)
	if err != nil {
		return 0, err
	}
	return snapshot.MixVolumeL, nil
}

// ServingSizeForVolume converts a draw volume to serving grams.
func (r *Recipe) ServingSizeForVolume(portionL float64, opts MixOptions) (float64, error) {
	snapshot, err := r.mixSnapshotWithOptions(opts)
	if err != nil {
		return 0, err
	}
	if snapshot.MixVolumeL <= 0 {
		return 0, errors.New("mix volume is zero")
	}
	density := snapshot.TotalMassKg / (snapshot.MixVolumeL * (1.0 + r.Overrun))
	return density * portionL * 1000.0, nil
}

// NutritionFacts computes per-serving nutrition.
func (r *Recipe) NutritionFacts(servingSizeGrams float64, sodiumMg float64) (NutritionFacts, error) {
	snapshot, err := r.aggregateTotals()
	if err != nil {
		return NutritionFacts{}, err
	}
	return snapshot.NutritionFactsSummary(servingSizeGrams, sodiumMg)
}
