package creamery

import "sync"

const (
	LabelBenAndJerryVanilla = "ben"
	LabelJenisSweetCream    = "jenis"
	LabelHaagenDazsVanilla  = "haagen"
	LabelBrighamsVanilla    = "brighams"
	LabelBreyersVanilla     = "breyers"
	LabelTalentiVanilla     = "talenti"
)

const labelDataDir = "labels"

// LabelScenarioDefinition captures canonical data for a label reconstruction.
type LabelScenarioDefinition struct {
	Key             string
	Name            string
	Label           NutritionLabel
	Facts           NutritionFacts
	DisplayNames    []string
	Lots            []LotDescriptor
	Batches         map[IngredientID]LotDescriptor
	IngredientSpecs []IngredientDefinition
	ScenarioSpecs   []IngredientDefinition
	Presence        []IngredientID
	Groups          []LabelGroup
	PintMassGrams   float64
	ServeTempC      float64
	DrawTempC       float64
	ShearRate       float64
	OverrunCap      *float64
}

var (
	labelDefsOnce sync.Once
	labelDefs     map[string]LabelScenarioDefinition
	fdaLabels     map[string]Label
)

// LabelScenarioByKey returns the canonical definition for the requested key.
func LabelScenarioByKey(key string) (LabelScenarioDefinition, bool) {
	labelDefsOnce.Do(loadLabelDefinitions)
	def, ok := labelDefs[key]
	return def, ok
}

// LabelDefinitionByKey exposes the same data for higher-level tests.
func LabelDefinitionByKey(key string) (LabelScenarioDefinition, bool) {
	return LabelScenarioByKey(key)
}

// FDALabelByKey returns the parsed FDA label for the given key.
func FDALabelByKey(key string) (Label, bool) {
	labelDefsOnce.Do(loadLabelDefinitions)
	label, ok := fdaLabels[key]
	return label, ok
}

// AllFDALabels returns all parsed FDA labels.
func AllFDALabels() map[string]Label {
	labelDefsOnce.Do(loadLabelDefinitions)
	result := make(map[string]Label, len(fdaLabels))
	for k, v := range fdaLabels {
		result[k] = v
	}
	return result
}

func loadLabelDefinitions() {
	catalog := DefaultIngredientCatalog()

	// Load from .fda files
	labels, err := LoadLabelsFromDir(labelDataDir)
	if err == nil && len(labels) > 0 {
		fdaLabels = labels
		labelDefs = make(map[string]LabelScenarioDefinition, len(labels))
		for key, label := range labels {
			def, err := label.ToScenarioDefinition(catalog)
			if err != nil {
				continue
			}
			labelDefs[key] = def
		}
		return
	}

	// Fallback: empty maps
	fdaLabels = make(map[string]Label)
	labelDefs = make(map[string]LabelScenarioDefinition)
}

func nutritionLabelFromFacts(facts NutritionFacts) NutritionLabel {
	return NutritionLabel{
		ServingSize: facts.ServingSizeGrams,
		Calories:    facts.Calories,
		TotalFat:    facts.TotalFatGrams,
		TotalCarbs:  facts.TotalCarbGrams,
		Sugars:      facts.TotalSugarsGrams,
		AddedSugars: facts.AddedSugarsGrams,
		Protein:     facts.ProteinGrams,
	}
}
