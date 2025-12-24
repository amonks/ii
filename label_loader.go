package creamery

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
)

const labelDataFile = "labels.json"

func loadLabelDefinitionsFromFile(path string) (map[string]LabelScenarioDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []labelFileDefinition
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(entries) == 0 {
		return nil, errors.New("label file contained no entries")
	}

	catalog := DefaultIngredientCatalog()
	defs := make(map[string]LabelScenarioDefinition, len(entries))
	for _, entry := range entries {
		def, err := entry.toDefinition(catalog)
		if err != nil {
			return nil, fmt.Errorf("%s (%s): %w", entry.Name, entry.ID, err)
		}
		defs[def.Key] = def
	}
	return defs, nil
}

type labelFileDefinition struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Facts        labelFactsData        `json:"facts"`
	PintMass     float64               `json:"pint_mass_g"`
	ServeTemp    float64               `json:"serve_temp_c"`
	DrawTemp     float64               `json:"draw_temp_c"`
	ShearRate    float64               `json:"shear_rate"`
	OverrunCap   *float64              `json:"overrun_cap"`
	DisplayNames []string              `json:"display_names"`
	Ingredients  []labelIngredientSpec `json:"ingredients"`
	Specs        []labelSpecDefinition `json:"specs"`
	Presence     []string              `json:"presence"`
	Groups       []labelGroupSpec      `json:"groups"`
}

type labelFactsData struct {
	ServingSize float64 `json:"serving_size_g"`
	Calories    float64 `json:"calories"`
	TotalFat    float64 `json:"total_fat_g"`
	SatFat      float64 `json:"sat_fat_g"`
	TransFat    float64 `json:"trans_fat_g"`
	TotalCarb   float64 `json:"total_carb_g"`
	TotalSugars float64 `json:"total_sugars_g"`
	AddedSugars float64 `json:"added_sugars_g"`
	Protein     float64 `json:"protein_g"`
	Sodium      float64 `json:"sodium_mg"`
	Cholesterol float64 `json:"cholesterol_mg"`
}

type labelIngredientSpec struct {
	Source      string             `json:"source"`
	ID          string             `json:"id"`
	DisplayName string             `json:"display_name"`
	Components  map[string]float64 `json:"components"`
	Label       string             `json:"label"`
}

type labelSpecDefinition struct {
	Source string `json:"source"`
	Name   string `json:"name"`
}

type labelGroupSpec struct {
	Name           string                         `json:"name"`
	Members        []string                       `json:"members"`
	FractionBounds map[string]labelFractionBounds `json:"fraction_bounds"`
	EnforceOrder   bool                           `json:"enforce_order"`
	InternalOrder  bool                           `json:"internal_order"`
}

type labelFractionBounds struct {
	Lo *float64 `json:"lo"`
	Hi *float64 `json:"hi"`
}

func (entry labelFileDefinition) toDefinition(catalog IngredientCatalog) (LabelScenarioDefinition, error) {
	if entry.ID == "" || entry.Name == "" {
		return LabelScenarioDefinition{}, errors.New("missing id or name")
	}
	if len(entry.Ingredients) == 0 {
		return LabelScenarioDefinition{}, errors.New("label has no ingredients")
	}

	lots := make([]LotDescriptor, 0, len(entry.Ingredients))
	batches := make(map[IngredientID]LotDescriptor, len(entry.Ingredients))
	specs := make([]IngredientDefinition, 0, len(entry.Ingredients))
	for _, ing := range entry.Ingredients {
		lot, err := buildIngredientClone(ing, catalog)
		if err != nil {
			return LabelScenarioDefinition{}, err
		}
		lots = append(lots, lot)
		if lot.Definition != nil {
			batches[lot.Definition.ID] = lot
			specs = append(specs, *lot.Definition)
		}
	}

	presence := make([]IngredientID, 0, len(entry.Presence))
	for _, name := range entry.Presence {
		presence = append(presence, NewIngredientID(name))
	}

	groups := make([]LabelGroup, 0, len(entry.Groups))
	for _, g := range entry.Groups {
		if len(g.Members) == 0 {
			continue
		}
		group := LabelGroup{
			Name:                 g.Name,
			Keys:                 make([]IngredientID, 0, len(g.Members)),
			EnforceInternalOrder: g.InternalOrder,
		}
		for _, member := range g.Members {
			group.Keys = append(group.Keys, NewIngredientID(member))
		}
		if len(g.FractionBounds) > 0 {
			group.FractionBounds = make(map[IngredientID]Interval, len(g.FractionBounds))
			for member, bounds := range g.FractionBounds {
				group.FractionBounds[NewIngredientID(member)] = fractionInterval(bounds)
			}
		}
		groups = append(groups, group)
	}

	facts := NutritionFacts{
		ServingSizeGrams:  entry.Facts.ServingSize,
		Calories:          entry.Facts.Calories,
		TotalFatGrams:     entry.Facts.TotalFat,
		SaturatedFatGrams: entry.Facts.SatFat,
		TransFatGrams:     entry.Facts.TransFat,
		TotalCarbGrams:    entry.Facts.TotalCarb,
		TotalSugarsGrams:  entry.Facts.TotalSugars,
		AddedSugarsGrams:  entry.Facts.AddedSugars,
		ProteinGrams:      entry.Facts.Protein,
		SodiumMg:          entry.Facts.Sodium,
		CholesterolMg:     entry.Facts.Cholesterol,
	}

	displaySpecs := specs
	if len(entry.Specs) > 0 {
		overrides, err := buildDisplaySpecs(entry.Specs, catalog)
		if err != nil {
			return LabelScenarioDefinition{}, err
		}
		displaySpecs = overrides
	}

	def := LabelScenarioDefinition{
		Key:             entry.ID,
		Name:            entry.Name,
		Label:           nutritionLabelFromFacts(facts),
		Facts:           facts,
		DisplayNames:    entry.DisplayNames,
		Lots:            lots,
		Batches:         batches,
		IngredientSpecs: displaySpecs,
		ScenarioSpecs:   specs,
		Presence:        presence,
		Groups:          groups,
		PintMassGrams:   entry.PintMass,
		ServeTempC:      entry.ServeTemp,
		DrawTempC:       entry.DrawTemp,
		ShearRate:       entry.ShearRate,
		OverrunCap:      entry.OverrunCap,
	}
	return def, nil
}

func buildIngredientClone(spec labelIngredientSpec, catalog IngredientCatalog) (LotDescriptor, error) {
	inst, ok := catalog.InstanceByKey(spec.Source)
	if !ok {
		return LotDescriptor{}, fmt.Errorf("unknown ingredient %q", spec.Source)
	}
	cloneID := spec.ID
	if cloneID == "" {
		cloneID = spec.Source
	}
	inst = renameInstance(inst, cloneID)
	if spec.DisplayName != "" && inst.Definition != nil {
		def := *inst.Definition
		def.Name = spec.DisplayName
		def.Profile.Name = spec.DisplayName
		inst = inst.WithSpec(def)
	}
	if inst.Definition != nil {
		if len(spec.Components) > 0 {
			def := *inst.Definition
			overrideComponents(&def.Profile, spec.Components)
			inst = inst.WithSpec(def)
		}
		if spec.Label != "" {
			inst.Label = spec.Label
		} else if inst.Label == "" && inst.Definition != nil {
			inst.Label = inst.Definition.Name
		}
	}
	return inst, nil
}

func overrideComponents(profile *ConstituentProfile, components map[string]float64) {
	for key, value := range components {
		switch key {
		case "water":
			profile.Components.Water = Point(value)
		case "fat":
			profile.Components.Fat = Point(value)
		case "protein":
			profile.Components.Protein = Point(value)
		case "lactose":
			profile.Components.Lactose = Point(value)
		case "sucrose":
			profile.Components.Sucrose = Point(value)
		case "glucose":
			profile.Components.Glucose = Point(value)
		case "fructose":
			profile.Components.Fructose = Point(value)
		case "maltodextrin":
			profile.Components.Maltodextrin = Point(value)
		case "polyols":
			profile.Components.Polyols = Point(value)
		case "other_solids", "other":
			profile.Components.OtherSolids = Point(value)
		}
	}
}

func fractionInterval(bounds labelFractionBounds) Interval {
	switch {
	case bounds.Lo != nil && bounds.Hi != nil:
		return RangeWithEps(*bounds.Lo, *bounds.Hi)
	case bounds.Lo != nil:
		return Interval{Lo: *bounds.Lo, Hi: math.Inf(1)}
	case bounds.Hi != nil:
		return Interval{Lo: 0, Hi: *bounds.Hi}
	default:
		return Interval{}
	}
}

func buildDisplaySpecs(entries []labelSpecDefinition, catalog IngredientCatalog) ([]IngredientDefinition, error) {
	specs := make([]IngredientDefinition, 0, len(entries))
	for _, spec := range entries {
		base, err := specDefinitionBySource(spec.Source, catalog)
		if err != nil {
			return nil, err
		}
		if spec.Name != "" {
			base = specWithName(base, spec.Name)
		}
		specs = append(specs, base)
	}
	return specs, nil
}

func specDefinitionBySource(source string, catalog IngredientCatalog) (IngredientDefinition, error) {
	switch source {
	case "":
		return IngredientDefinition{}, errors.New("missing spec source")
	case "nonfat_milk_variable":
		return NonfatMilkVariable, nil
	default:
		def, err := specFromCatalog(source)
		if err != nil {
			return IngredientDefinition{}, err
		}
		return def, nil
	}
}
