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

var (
	labelDefsOnce sync.Once
	fdaLabels     map[string]Label
)

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
	labels, err := LoadLabelsFromDir(labelDataDir)
	if err == nil && len(labels) > 0 {
		fdaLabels = labels
		return
	}
	fdaLabels = make(map[string]Label)
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
