package creamery

// specFromCatalog builds an IngredientDefinition from the default catalog, overriding
// the display name when provided.
func specFromCatalog(key, displayName string) IngredientDefinition {
	inst, ok := DefaultIngredientCatalog().InstanceByKey(key)
	if !ok {
		if displayName == "" {
			displayName = key
		}
		spec := SpecFromComposition(displayName, Composition{})
		spec.Key = NewIngredientKey(key)
		return normalizeSpec(spec)
	}
	def := inst.Definition
	if def == nil {
		spec := SpecFromComposition(displayName, Composition{})
		spec.Key = NewIngredientKey(key)
		return normalizeSpec(spec)
	}
	spec := *def
	if displayName != "" {
		spec = renameSpec(spec, displayName)
	}
	return spec
}

func renameSpec(spec IngredientDefinition, name string) IngredientDefinition {
	key := spec.Key
	spec.Name = name
	spec.ID = NewIngredientID(name)
	spec.Profile.Name = name
	spec.Profile.ID = spec.ID
	spec = normalizeSpec(spec)
	if key != "" {
		spec.Key = key
	}
	return spec
}

// Standard ingredient specifications with typical compositions.
var (
	HeavyCream = specFromCatalog("heavy_cream", "Heavy Cream")
	LightCream = specFromCatalog("light_cream", "Light Cream")
	WholeMilk  = specFromCatalog("whole_milk", "Whole Milk")
	SkimMilk   = specFromCatalog("skim_milk", "Skim Milk")

	NonfatDryMilk          = specFromCatalog("skim_milk_powder", "Nonfat Dry Milk")
	SweetenedCondensedMilk = specFromCatalog("sweetened_condensed_milk", "Sweetened Condensed Milk")
	Butter                 = specFromCatalog("butter", "Butter")
	EggYolks               = specFromCatalog("egg_yolk", "Egg Yolks")
	Sugar                  = specFromCatalog("sucrose", "Sugar")
	CornSyrup              = specFromCatalog("corn_syrup_42", "Corn Syrup")
	LiquidSugar            = specFromCatalog("liquid_sugar", "Liquid Sugar")
	CocoaPowder            = specFromCatalog("cocoa_powder", "Cocoa Powder")
	VanillaExtract         = specFromCatalog("vanilla_extract", "Vanilla Extract")
	Stabilizer             = specFromCatalog("stabilizer", "Stabilizer")
	TapiocaSyrup           = specFromCatalog("tapioca_syrup", "Tapioca Syrup")

	NonfatMilkVariable = SpecFromComposition("Nonfat Milk", Composition{
		Fat:   Range(0, 0.005),
		MSNF:  Range(0.09, 0.97),
		Sugar: Point(0),
		Other: Point(0),
	})
)

// StandardSpecs returns a slice of commonly used ingredient specs.
func StandardSpecs() []IngredientDefinition {
	catalog := DefaultIngredientCatalog()
	specs := make([]IngredientDefinition, 0, len(standardSpecKeys)+1)
	for _, entry := range standardSpecKeys {
		inst, ok := catalog.InstanceByKey(entry.key)
		if !ok {
			continue
		}
		if inst.Definition == nil {
			continue
		}
		spec := *inst.Definition
		if entry.display != "" {
			spec = renameSpec(spec, entry.display)
		}
		specs = append(specs, spec)
	}
	specs = append(specs, NonfatMilkVariable)
	return specs
}

// StandardSpecMap provides the same specs keyed by their IngredientID.
func StandardSpecMap() map[IngredientID]IngredientDefinition {
	specs := StandardSpecs()
	lib := make(map[IngredientID]IngredientDefinition, len(specs))
	for _, spec := range specs {
		lib[spec.ID] = spec
	}
	return lib
}

var standardSpecKeys = []struct {
	key     string
	display string
}{
	{"heavy_cream", "Heavy Cream"},
	{"light_cream", "Light Cream"},
	{"whole_milk", "Whole Milk"},
	{"skim_milk", "Skim Milk"},
	{"skim_milk_powder", "Nonfat Dry Milk"},
	{"sweetened_condensed_milk", "Sweetened Condensed Milk"},
	{"butter", "Butter"},
	{"egg_yolk", "Egg Yolks"},
	{"sucrose", "Sugar"},
	{"corn_syrup_42", "Corn Syrup"},
	{"liquid_sugar", "Liquid Sugar"},
	{"cocoa_powder", "Cocoa Powder"},
	{"vanilla_extract", "Vanilla Extract"},
	{"stabilizer", "Stabilizer"},
	{"tapioca_syrup", "Tapioca Syrup"},
}
