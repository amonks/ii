package fdaparser

import (
	"os"
	"strconv"
	"strings"
)

// Label represents an FDA nutrition label parsed from .fda format.
type Label struct {
	ID            string
	Name          string
	PintMassGrams float64
	Facts         LabelFacts
	Ingredients   []LabelIngredient
	Groups        []FDAGroup
}

// LabelIngredient represents an ingredient in an FDA label.
type LabelIngredient struct {
	ID         string
	Components map[string]float64
}

// FDAGroup represents a group of ingredients (like "Cream" containing cream_fat and cream_serum).
type FDAGroup struct {
	Name           string
	Members        []string
	FractionBounds map[string]FDAFractionBound
	EnforceOrder   bool
}

// FDAFractionBound represents a fraction constraint on an ingredient within a group.
type FDAFractionBound struct {
	Lo float64
	Hi float64
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

func (p *fdaParser) startGroup(name string) {
	p.currentGroup = &FDAGroup{Name: strings.TrimSpace(name)}
}

func (p *fdaParser) setFractionBound(hi float64) {
	if p.currentGroup.FractionBounds == nil {
		p.currentGroup.FractionBounds = make(map[string]FDAFractionBound)
	}
	p.currentGroup.FractionBounds[p.boundKey] = FDAFractionBound{Lo: p.boundLo, Hi: hi}
}

func (p *fdaParser) setGroupOrder() {
	p.currentGroup.EnforceOrder = true
}

func (p *fdaParser) addSubIngredient(id string) {
	p.currentGroup.Members = append(p.currentGroup.Members, id)
	p.label.Ingredients = append(p.label.Ingredients, LabelIngredient{ID: id})
}

func (p *fdaParser) finishGroup() {
	p.label.Groups = append(p.label.Groups, *p.currentGroup)
	p.currentGroup = nil
}

func (p *fdaParser) startIngredient(id string) {
	p.currentIngredient = &LabelIngredient{ID: id}
}

func (p *fdaParser) setComponent(value float64) {
	if p.currentIngredient.Components == nil {
		p.currentIngredient.Components = make(map[string]float64)
	}
	p.currentIngredient.Components[p.componentKey] = value
}

func (p *fdaParser) finishIngredient() {
	p.label.Ingredients = append(p.label.Ingredients, *p.currentIngredient)
	p.currentIngredient = nil
}

// Parse parses an FDA label from the given content string.
func Parse(content string) (Label, error) {
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

// ParseFile parses an FDA label from a file path.
func ParseFile(path string) (Label, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Label{}, err
	}
	return Parse(string(data))
}
