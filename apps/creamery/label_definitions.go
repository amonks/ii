package creamery

import (
	"maps"
	"sync"

	"monks.co/apps/creamery/fdaparser"
)

const (
	LabelBenAndJerryVanilla = "ben"
	LabelJenisSweetCream    = "jenis"
	LabelHaagenDazsVanilla  = "haagen"
	LabelBrighamsVanilla    = "brighams"
	LabelBreyersVanilla     = "breyers"
	LabelTalentiVanilla     = "talenti"
)

const labelDataDir = "labels"

var (
	labelDefsOnce sync.Once
	fdaLabels     map[string]fdaparser.Label
)

// FDALabelByKey returns the parsed FDA label for the given key.
func FDALabelByKey(key string) (fdaparser.Label, bool) {
	labelDefsOnce.Do(loadLabelDefinitions)
	label, ok := fdaLabels[key]
	return label, ok
}

// AllFDALabels returns all parsed FDA labels.
func AllFDALabels() map[string]fdaparser.Label {
	labelDefsOnce.Do(loadLabelDefinitions)
	result := make(map[string]fdaparser.Label, len(fdaLabels))
	maps.Copy(result, fdaLabels)
	return result
}

func loadLabelDefinitions() {
	labels, err := LoadLabelsFromDir(labelDataDir)
	if err == nil && len(labels) > 0 {
		fdaLabels = labels
		return
	}
	fdaLabels = make(map[string]fdaparser.Label)
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
