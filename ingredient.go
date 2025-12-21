package creamery

// specFromBatch builds an Ingredient from a detailed batch entry, overriding
// the display name when provided.
func specFromBatch(key, displayName string) Ingredient {
	batch, ok := IngredientBatchTable()[key]
	if !ok {
		return SpecFromComposition(displayName, Composition{})
	}
	if displayName != "" {
		batch.Name = displayName
	}
	return batch.ToSpec()
}

// Standard ingredient specifications with typical compositions.
var (
	HeavyCream = SpecFromComposition("Heavy Cream", Composition{
		Fat:   Range(0.36, 0.40),
		MSNF:  Range(0.05, 0.06),
		Sugar: Point(0),
		Other: Point(0),
	})

	LightCream = SpecFromComposition("Light Cream", Composition{
		Fat:   Range(0.18, 0.30),
		MSNF:  Range(0.06, 0.08),
		Sugar: Point(0),
		Other: Point(0),
	})

	WholeMilk = SpecFromComposition("Whole Milk", Composition{
		Fat:   Range(0.032, 0.035),
		MSNF:  Range(0.085, 0.09),
		Sugar: Point(0),
		Other: Point(0),
	})

	SkimMilk = SpecFromComposition("Skim Milk", Composition{
		Fat:   Range(0, 0.005),
		MSNF:  Range(0.09, 0.095),
		Sugar: Point(0),
		Other: Point(0),
	})

	NonfatDryMilk = SpecFromComposition("Nonfat Dry Milk", Composition{
		Fat:   Range(0.005, 0.015),
		MSNF:  Range(0.95, 0.97),
		Sugar: Point(0),
		Other: Point(0),
	})

	SweetenedCondensedMilk = SpecFromComposition("Sweetened Condensed Milk", Composition{
		Fat:   Range(0.08, 0.09),
		MSNF:  Range(0.19, 0.21),
		Sugar: Range(0.43, 0.47),
		Other: Point(0),
	})

	Butter = SpecFromComposition("Butter", Composition{
		Fat:   Range(0.80, 0.82),
		MSNF:  Range(0.01, 0.02),
		Sugar: Point(0),
		Other: Point(0),
	})

	EggYolks = SpecFromComposition("Egg Yolks", Composition{
		Fat:   Range(0.30, 0.33),
		MSNF:  Point(0),
		Sugar: Point(0),
		Other: Range(0.16, 0.18),
	})

	Sugar = SpecFromComposition("Sugar", Composition{
		Fat:   Point(0),
		MSNF:  Point(0),
		Sugar: Point(1.0),
		Other: Point(0),
	})

	CornSyrup   = specFromBatch("corn_syrup_42", "Corn Syrup")
	LiquidSugar = SpecFromComposition("Liquid Sugar", Composition{
		Fat:   Point(0),
		MSNF:  Point(0),
		Sugar: Range(0.65, 0.68),
		Other: Point(0),
	})
	CocoaPowder = SpecFromComposition("Cocoa Powder", Composition{
		Fat:   Range(0.10, 0.24),
		MSNF:  Point(0),
		Sugar: Point(0),
		Other: Range(0.70, 0.85),
	})
	VanillaExtract = SpecFromComposition("Vanilla Extract", Composition{
		Fat:   Point(0),
		MSNF:  Point(0),
		Sugar: Point(0),
		Other: Range(0, 0.02),
	})
	Stabilizer = SpecFromComposition("Stabilizer", Composition{
		Fat:   Point(0),
		MSNF:  Point(0),
		Sugar: Point(0),
		Other: Point(1.0),
	})
	TapiocaSyrup = specFromBatch("tapioca_syrup", "Tapioca Syrup")

	NonfatMilkVariable = SpecFromComposition("Nonfat Milk", Composition{
		Fat:   Range(0, 0.005),
		MSNF:  Range(0.09, 0.97),
		Sugar: Point(0),
		Other: Point(0),
	})
)

// StandardSpecs returns a slice of commonly used ingredient specs.
func StandardSpecs() []Ingredient {
	return []Ingredient{
		HeavyCream,
		LightCream,
		WholeMilk,
		SkimMilk,
		NonfatDryMilk,
		SweetenedCondensedMilk,
		Butter,
		EggYolks,
		Sugar,
		CornSyrup,
		LiquidSugar,
		CocoaPowder,
		VanillaExtract,
		Stabilizer,
		TapiocaSyrup,
		NonfatMilkVariable,
	}
}

// StandardSpecMap provides the same specs keyed by their IngredientID.
func StandardSpecMap() map[IngredientID]Ingredient {
	specs := StandardSpecs()
	lib := make(map[IngredientID]Ingredient, len(specs))
	for _, spec := range specs {
		lib[spec.ID] = spec
	}
	return lib
}
