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

func loadLabelDefinitions() {
	if defs, err := loadLabelDefinitionsFromFile(labelDataFile); err == nil && len(defs) > 0 {
		labelDefs = defs
		return
	}
	labelDefs = map[string]LabelScenarioDefinition{
		LabelBenAndJerryVanilla: defineBenAndJerryVanilla(),
		LabelJenisSweetCream:    defineJenisSweetCream(),
		LabelHaagenDazsVanilla:  defineHaagenDazsVanilla(),
		LabelBrighamsVanilla:    defineBrighamsVanilla(),
		LabelBreyersVanilla:     defineBreyersVanilla(),
		LabelTalentiVanilla:     defineTalentiVanilla(),
	}
}

func defineBenAndJerryVanilla() LabelScenarioDefinition {
	builder := newScenarioIngredients()
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("skim_milk", "skim_milk", nil)
	builder.addClone("water", "water", nil)
	builder.addClone("egg_yolk", "egg_yolk", nil)
	builder.addClone("sucrose", "sucrose", nil)
	builder.addClone("guar_gum", "guar_gum", nil)
	builder.addClone("vanilla_extract", "vanilla_extract", nil)
	builder.addClone("vanilla_beans", "vanilla_beans", nil)
	builder.addClone("carrageenan", "carrageenan", nil)
	builder.addClone("sucrose", "liquid_sugar_sucrose", nil)
	builder.addClone("water", "liquid_sugar_water", nil)

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
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "skim_milk", Keys: []IngredientID{builder.id("skim_milk")}},
		{
			Name:                 "liquid_sugar",
			Keys:                 []IngredientID{builder.id("liquid_sugar_sucrose"), builder.id("liquid_sugar_water")},
			EnforceInternalOrder: true,
		},
		{Name: "water", Keys: []IngredientID{builder.id("water")}},
		{Name: "egg_yolk", Keys: []IngredientID{builder.id("egg_yolk")}},
		{Name: "sucrose", Keys: []IngredientID{builder.id("sucrose")}},
		{Name: "guar_gum", Keys: []IngredientID{builder.id("guar_gum")}},
		{Name: "vanilla_extract", Keys: []IngredientID{builder.id("vanilla_extract")}},
		{Name: "vanilla_beans", Keys: []IngredientID{builder.id("vanilla_beans")}},
		{Name: "carrageenan", Keys: []IngredientID{builder.id("carrageenan")}},
	}
	presence := builder.idList("cream_fat", "cream_serum", "skim_milk", "liquid_sugar_sucrose", "liquid_sugar_water", "water", "egg_yolk", "sucrose", "guar_gum", "vanilla_extract", "vanilla_beans", "carrageenan")
	labelIngredients := []string{"cream", "skim milk", "liquid sugar (sucrose, water)", "water", "egg yolks", "sugar", "guar gum", "vanilla extract", "vanilla beans", "carrageenan"}

	return LabelScenarioDefinition{
		Key:             LabelBenAndJerryVanilla,
		Name:            "Ben & Jerry's Vanilla",
		Label:           nutritionLabelFromFacts(facts),
		Facts:           facts,
		DisplayNames:    labelIngredients,
		Lots:            builder.Lots(),
		Batches:         builder.Batches(),
		IngredientSpecs: builder.Specs(),
		ScenarioSpecs:   builder.Specs(),
		Presence:        presence,
		Groups:          groups,
		PintMassGrams:   430.0,
	}
}

func defineJenisSweetCream() LabelScenarioDefinition {
	builder := newScenarioIngredients()
	builder.addClone("milk", "milk", nil)
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("sucrose", "cane_sugar", nil)
	builder.addClone("skim_milk", "nonfat_milk", nil)
	builder.addClone("tapioca_syrup", "tapioca_syrup", nil)

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
		{Name: "milk", Keys: []IngredientID{builder.id("milk")}},
		{
			Name: "cream",
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "cane_sugar", Keys: []IngredientID{builder.id("cane_sugar")}},
		{Name: "nonfat_milk", Keys: []IngredientID{builder.id("nonfat_milk")}},
		{Name: "tapioca_syrup", Keys: []IngredientID{builder.id("tapioca_syrup")}},
	}
	presence := builder.idList("milk", "cream_fat", "cream_serum", "cane_sugar", "nonfat_milk", "tapioca_syrup")
	labelIngredients := []string{"milk", "cream", "cane sugar", "nonfat milk", "tapioca syrup"}
	ingredientSpecs := []IngredientDefinition{
		specWithName(WholeMilk, "Milk"),
		specWithName(HeavyCream, "Cream"),
		specWithName(Sugar, "Cane Sugar"),
		specWithName(NonfatMilkVariable, "Nonfat Milk"),
		specWithName(TapiocaSyrup, "Tapioca Syrup"),
	}

	return LabelScenarioDefinition{
		Key:             LabelJenisSweetCream,
		Name:            "Jeni's Sweet Cream",
		Label:           nutritionLabelFromFacts(facts),
		Facts:           facts,
		DisplayNames:    labelIngredients,
		Lots:            builder.Lots(),
		Batches:         builder.Batches(),
		IngredientSpecs: ingredientSpecs,
		ScenarioSpecs:   builder.Specs(),
		Presence:        presence,
		Groups:          groups,
		PintMassGrams:   facts.ServingSizeGrams * 3,
	}
}

func defineHaagenDazsVanilla() LabelScenarioDefinition {
	builder := newScenarioIngredients()
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("skim_milk", "skim_milk", nil)
	builder.addClone("sucrose", "cane_sugar", nil)
	builder.addClone("egg_yolk", "egg_yolk", nil)
	builder.addClone("vanilla_extract", "vanilla_extract", nil)

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
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "skim_milk", Keys: []IngredientID{builder.id("skim_milk")}},
		{Name: "cane_sugar", Keys: []IngredientID{builder.id("cane_sugar")}},
		{Name: "egg_yolk", Keys: []IngredientID{builder.id("egg_yolk")}},
		{Name: "vanilla_extract", Keys: []IngredientID{builder.id("vanilla_extract")}},
	}
	presence := builder.idList("cream_fat", "cream_serum", "skim_milk", "cane_sugar", "egg_yolk", "vanilla_extract")
	labelIngredients := []string{"cream", "skim milk", "cane sugar", "egg yolks", "vanilla extract"}
	ingredientSpecs := []IngredientDefinition{
		specWithName(HeavyCream, "Cream"),
		specWithName(SkimMilk, "Skim Milk"),
		specWithName(Sugar, "Cane Sugar"),
		specWithName(EggYolks, "Egg Yolks"),
		specWithName(VanillaExtract, "Vanilla Extract"),
	}

	return LabelScenarioDefinition{
		Key:             LabelHaagenDazsVanilla,
		Name:            "Haagen-Dazs Vanilla",
		Label:           nutritionLabelFromFacts(facts),
		Facts:           facts,
		DisplayNames:    labelIngredients,
		Lots:            builder.Lots(),
		Batches:         builder.Batches(),
		IngredientSpecs: ingredientSpecs,
		ScenarioSpecs:   builder.Specs(),
		Presence:        presence,
		Groups:          groups,
		PintMassGrams:   facts.ServingSizeGrams * 3,
	}
}

func defineBrighamsVanilla() LabelScenarioDefinition {
	builder := newScenarioIngredients()
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("milk", "milk", nil)
	builder.addClone("sucrose", "sugar", nil)
	builder.addClone("vanilla_extract", "vanilla_extract", nil)
	builder.addClone("guar_gum", "guar_gum", nil)
	builder.addClone("salt", "salt", nil)
	builder.addClone("mono_diglycerides", "mono_diglycerides", nil)
	builder.addClone("ps80", "ps80", nil)
	builder.addClone("carrageenan", "carrageenan", nil)
	builder.addClone("potassium_phosphate", "potassium_phosphate", nil)
	builder.addClone("xanthan", "cellulose_gum", nil)

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
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "milk", Keys: []IngredientID{builder.id("milk")}},
		{Name: "sugar", Keys: []IngredientID{builder.id("sugar")}},
		{Name: "vanilla_extract", Keys: []IngredientID{builder.id("vanilla_extract")}},
		{Name: "guar_gum", Keys: []IngredientID{builder.id("guar_gum")}},
		{Name: "salt", Keys: []IngredientID{builder.id("salt")}},
		{Name: "mono_diglycerides", Keys: []IngredientID{builder.id("mono_diglycerides")}},
		{Name: "ps80", Keys: []IngredientID{builder.id("ps80")}},
		{Name: "carrageenan", Keys: []IngredientID{builder.id("carrageenan")}},
		{Name: "potassium_phosphate", Keys: []IngredientID{builder.id("potassium_phosphate")}},
		{Name: "cellulose_gum", Keys: []IngredientID{builder.id("cellulose_gum")}},
	}
	presence := builder.idList("cream_fat", "cream_serum", "milk", "sugar", "vanilla_extract", "guar_gum", "salt", "mono_diglycerides", "ps80", "carrageenan", "potassium_phosphate", "cellulose_gum")
	labelIngredients := []string{"cream", "milk", "sugar", "vanilla extract", "guar gum", "salt", "mono & diglycerides", "ps80", "carrageenan", "potassium phosphate", "cellulose gum"}

	return LabelScenarioDefinition{
		Key:             LabelBrighamsVanilla,
		Name:            "Brigham's Vanilla",
		Label:           nutritionLabelFromFacts(facts),
		Facts:           facts,
		DisplayNames:    labelIngredients,
		Lots:            builder.Lots(),
		Batches:         builder.Batches(),
		IngredientSpecs: builder.Specs(),
		ScenarioSpecs:   builder.Specs(),
		Presence:        presence,
		Groups:          groups,
		PintMassGrams:   facts.ServingSizeGrams * 3,
	}
}

func defineBreyersVanilla() LabelScenarioDefinition {
	builder := newScenarioIngredients()
	builder.addClone("milk", "milk", nil)
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("sucrose", "sugar", nil)
	builder.addClone("skim_milk", "skim_milk", nil)
	builder.addClone("tara_gum", "tara_gum", nil)
	builder.addClone("vanilla_extract", "natural_flavor", nil)

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
		{Name: "milk", Keys: []IngredientID{builder.id("milk")}},
		{
			Name: "cream",
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "sugar", Keys: []IngredientID{builder.id("sugar")}},
		{Name: "skim_milk", Keys: []IngredientID{builder.id("skim_milk")}},
		{Name: "tara_gum", Keys: []IngredientID{builder.id("tara_gum")}},
		{Name: "natural_flavor", Keys: []IngredientID{builder.id("natural_flavor")}},
	}
	presence := builder.idList("milk", "cream_fat", "cream_serum", "sugar", "skim_milk", "tara_gum", "natural_flavor")
	labelIngredients := []string{"milk", "cream", "sugar", "skim milk", "tara gum", "natural flavor"}

	return LabelScenarioDefinition{
		Key:             LabelBreyersVanilla,
		Name:            "Breyers Vanilla",
		Label:           nutritionLabelFromFacts(facts),
		Facts:           facts,
		DisplayNames:    labelIngredients,
		Lots:            builder.Lots(),
		Batches:         builder.Batches(),
		IngredientSpecs: builder.Specs(),
		ScenarioSpecs:   builder.Specs(),
		Presence:        presence,
		Groups:          groups,
		PintMassGrams:   facts.ServingSizeGrams * 3,
	}
}

func defineTalentiVanilla() LabelScenarioDefinition {
	builder := newScenarioIngredients()
	builder.addClone("milk", "milk", nil)
	builder.addClone("sucrose", "sugar", nil)
	builder.addClone("cream_fat", "cream_fat", nil)
	builder.addClone("cream_serum", "cream_serum", nil)
	builder.addClone("dextrose", "dextrose", nil)
	builder.addClone("vanilla_extract", "vanilla_extract", nil)
	builder.addClone("lecithin", "sunflower_lecithin", nil)
	builder.addClone("locust_bean_gum", "carob_bean_gum", nil)
	builder.addClone("guar_gum", "guar_gum", nil)
	builder.addClone("vanilla_extract", "natural_flavor", func(inst *LotDescriptor) {
		spec := IngredientDefinition{}
		if inst.Definition != nil {
			spec = *inst.Definition
		}
		profile := spec.Profile
		profile.Components.Water = Point(0.60)
		profile.Components.OtherSolids = Point(0.40)
		spec.Profile = profile
		*inst = inst.WithSpec(spec)
	})
	builder.addClone("lemon_peel", "lemon_peel", nil)

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
		{Name: "milk", Keys: []IngredientID{builder.id("milk")}},
		{Name: "sugar", Keys: []IngredientID{builder.id("sugar")}},
		{
			Name: "cream",
			Keys: []IngredientID{builder.id("cream_fat"), builder.id("cream_serum")},
			FractionBounds: map[IngredientID]Interval{
				builder.id("cream_fat"): RangeWithEps(0.18, 0.50),
			},
		},
		{Name: "dextrose", Keys: []IngredientID{builder.id("dextrose")}},
		{Name: "vanilla_extract", Keys: []IngredientID{builder.id("vanilla_extract")}},
		{Name: "sunflower_lecithin", Keys: []IngredientID{builder.id("sunflower_lecithin")}},
		{Name: "carob_bean_gum", Keys: []IngredientID{builder.id("carob_bean_gum")}},
		{Name: "guar_gum", Keys: []IngredientID{builder.id("guar_gum")}},
		{Name: "natural_flavor", Keys: []IngredientID{builder.id("natural_flavor")}},
		{Name: "lemon_peel", Keys: []IngredientID{builder.id("lemon_peel")}},
	}
	presence := builder.idList("milk", "sugar", "cream_fat", "cream_serum", "dextrose", "vanilla_extract", "sunflower_lecithin", "carob_bean_gum", "guar_gum", "natural_flavor", "lemon_peel")
	labelIngredients := []string{"milk", "sugar", "cream", "dextrose", "vanilla extract", "sunflower lecithin", "carob bean gum", "guar gum", "natural flavor", "lemon peel"}

	return LabelScenarioDefinition{
		Key:             LabelTalentiVanilla,
		Name:            "Talenti Vanilla Bean",
		Label:           nutritionLabelFromFacts(facts),
		Facts:           facts,
		DisplayNames:    labelIngredients,
		Lots:            builder.Lots(),
		Batches:         builder.Batches(),
		IngredientSpecs: builder.Specs(),
		ScenarioSpecs:   builder.Specs(),
		Presence:        presence,
		Groups:          groups,
		PintMassGrams:   facts.ServingSizeGrams * 3,
	}
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

func specWithName(base IngredientDefinition, name string) IngredientDefinition {
	spec := base
	if name != "" {
		spec.Name = name
		spec.ID = NewIngredientID(name)
		spec.Profile.Name = name
		spec.Profile.ID = spec.ID
	}
	return spec
}
