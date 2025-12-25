package creamery

import "strconv"

// Label represents an FDA nutrition label parsed from .fda format.
type Label struct {
	ID            string
	Name          string
	PintMassGrams float64
	Facts         LabelFacts
	Ingredients   []LabelIngredient
}

// LabelIngredient represents an ingredient in an FDA label.
type LabelIngredient struct {
	ID string
}

// LabelFacts contains nutrition facts from an FDA label.
type LabelFacts struct {
	ServingSizeGrams  float64
	Calories          float64
	TotalFatGrams     float64
	SaturatedFatGrams float64
	TransFatGrams     float64
	CholesterolMg     float64
	TotalCarbGrams    float64
	TotalSugarsGrams  float64
	AddedSugarsGrams  float64
	ProteinGrams      float64
	SodiumMg          float64
}

func (p *fdaParser) parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func (p *fdaParser) addIngredient(id string) {
	p.label.Ingredients = append(p.label.Ingredients, LabelIngredient{ID: id})
}

// ParseLabel parses an FDA label from the given content string.
func ParseLabel(content string) (Label, error) {
	p := &fdaParser{Buffer: content}
	if err := p.Init(); err != nil {
		return Label{}, err
	}
	if err := p.Parse(); err != nil {
		return Label{}, err
	}
	p.Execute()
	return p.label, nil
}
