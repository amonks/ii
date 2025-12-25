package creamery

import (
	"fmt"
	"os"
	"path/filepath"
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

// ParseLabelFile parses an FDA label from a file path.
func ParseLabelFile(path string) (Label, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Label{}, err
	}
	return ParseLabel(string(data))
}

// LoadLabelsFromDir loads all .fda files from a directory.
func LoadLabelsFromDir(dir string) (map[string]Label, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	labels := make(map[string]Label)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".fda") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		label, err := ParseLabelFile(path)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		labels[label.ID] = label
	}
	return labels, nil
}

// ToScenarioDefinition converts a Label to a LabelScenarioDefinition for backward compatibility.
func (l Label) ToScenarioDefinition(catalog IngredientCatalog) (LabelScenarioDefinition, error) {
	builder := newScenarioIngredients()
	builder.catalog = catalog

	// Build ingredients and lots from the label
	for _, ing := range l.Ingredients {
		source := ing.ID // ingredient ID is the source by default
		builder.addClone(source, ing.ID, func(inst *LotDescriptor) {
			if len(ing.Components) > 0 && inst.Definition != nil {
				def := *inst.Definition
				for key, value := range ing.Components {
					switch key {
					case "water":
						def.Profile.Components.Water = Point(value)
					case "fat":
						def.Profile.Components.Fat = Point(value)
					case "protein":
						def.Profile.Components.Protein = Point(value)
					case "lactose":
						def.Profile.Components.Lactose = Point(value)
					case "sucrose":
						def.Profile.Components.Sucrose = Point(value)
					case "glucose":
						def.Profile.Components.Glucose = Point(value)
					case "fructose":
						def.Profile.Components.Fructose = Point(value)
					case "other_solids":
						def.Profile.Components.OtherSolids = Point(value)
					}
				}
				*inst = inst.WithSpec(def)
			}
		})
	}

	// Convert Facts
	facts := NutritionFacts{
		ServingSizeGrams:  l.Facts.ServingSizeGrams,
		Calories:          l.Facts.Calories,
		TotalFatGrams:     l.Facts.TotalFatGrams,
		SaturatedFatGrams: l.Facts.SaturatedFatGrams,
		TransFatGrams:     l.Facts.TransFatGrams,
		CholesterolMg:     l.Facts.CholesterolMg,
		TotalCarbGrams:    l.Facts.TotalCarbGrams,
		TotalSugarsGrams:  l.Facts.TotalSugarsGrams,
		AddedSugarsGrams:  l.Facts.AddedSugarsGrams,
		ProteinGrams:      l.Facts.ProteinGrams,
		SodiumMg:          l.Facts.SodiumMg,
	}

	// Convert Groups
	groups := make([]LabelGroup, 0, len(l.Groups))
	for _, g := range l.Groups {
		keys := make([]IngredientID, len(g.Members))
		for i, member := range g.Members {
			keys[i] = NewIngredientID(member)
		}
		group := LabelGroup{
			Name:                 g.Name,
			Keys:                 keys,
			EnforceInternalOrder: g.EnforceOrder,
		}
		if len(g.FractionBounds) > 0 {
			group.FractionBounds = make(map[IngredientID]Interval)
			for key, bound := range g.FractionBounds {
				group.FractionBounds[NewIngredientID(key)] = RangeWithEps(bound.Lo, bound.Hi)
			}
		}
		groups = append(groups, group)
	}

	// For ingredients not in a group, create singleton groups
	inGroup := make(map[string]bool)
	for _, g := range l.Groups {
		for _, member := range g.Members {
			inGroup[member] = true
		}
	}
	for _, ing := range l.Ingredients {
		if !inGroup[ing.ID] {
			groups = append(groups, LabelGroup{
				Name: ing.ID,
				Keys: []IngredientID{NewIngredientID(ing.ID)},
			})
		}
	}

	// Build presence list
	presence := make([]IngredientID, len(l.Ingredients))
	for i, ing := range l.Ingredients {
		presence[i] = NewIngredientID(ing.ID)
	}

	return LabelScenarioDefinition{
		Key:             l.ID,
		Name:            l.Name,
		Label:           nutritionLabelFromFacts(facts),
		Facts:           facts,
		Lots:            builder.Lots(),
		Batches:         builder.Batches(),
		IngredientSpecs: builder.Specs(),
		ScenarioSpecs:   builder.Specs(),
		Presence:        presence,
		Groups:          groups,
		PintMassGrams:   l.PintMassGrams,
	}, nil
}
