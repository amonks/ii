package creamery

// specFromCatalog builds an IngredientDefinition from the default catalog.
func specFromCatalog(key string) IngredientDefinition {
	inst, ok := DefaultIngredientCatalog().InstanceByKey(key)
	if !ok {
		name := FriendlyIngredientName(key)
		profile := ConstituentProfile{
			ID:         NewIngredientID(name),
			Name:       name,
			Components: EnsureWater(ComponentFractions{}),
		}
		return SpecFromProfile(profile)
	}
	def := inst.Definition
	if def == nil {
		name := FriendlyIngredientName(key)
		profile := ConstituentProfile{
			ID:         NewIngredientID(name),
			Name:       name,
			Components: EnsureWater(ComponentFractions{}),
		}
		return SpecFromProfile(profile)
	}
	return *def
}

// Standard ingredient specifications with typical compositions.
var (
	HeavyCream = specFromCatalog("heavy_cream")
	LightCream = specFromCatalog("light_cream")
	WholeMilk  = specFromCatalog("whole_milk")
	SkimMilk   = specFromCatalog("skim_milk")

	NonfatDryMilk          = specFromCatalog("skim_milk_powder")
	SweetenedCondensedMilk = specFromCatalog("sweetened_condensed_milk")
	Butter                 = specFromCatalog("butter")
	EggYolks               = specFromCatalog("egg_yolk")
	Sugar                  = specFromCatalog("sucrose")
	CornSyrup              = specFromCatalog("corn_syrup_42")
	LiquidSugar            = specFromCatalog("liquid_sugar")
	CocoaPowder            = specFromCatalog("cocoa_powder")
	VanillaExtract         = specFromCatalog("vanilla_extract")
	Stabilizer             = specFromCatalog("stabilizer")
	TapiocaSyrup           = specFromCatalog("tapioca_syrup")

	NonfatMilkVariable = buildNonfatMilkVariable()
)

func buildNonfatMilkVariable() IngredientDefinition {
	name := "Nonfat Milk"
	components := ComponentFractions{
		Fat:         Range(0, 0.005),
		MSNF:        Range(0.09, 0.97),
		Sucrose:     Point(0),
		OtherSolids: Point(0),
	}
	components = EnsureWater(components)
	profile := ConstituentProfile{
		ID:         NewIngredientID(name),
		Name:       name,
		Components: components,
	}
	return SpecFromProfile(profile)
}

// StandardSpecs returns a slice of commonly used ingredient specs.
func StandardSpecs() []IngredientDefinition {
	catalog := DefaultIngredientCatalog()
	specs := make([]IngredientDefinition, 0, len(standardSpecKeys)+1)
	for _, key := range standardSpecKeys {
		inst, ok := catalog.InstanceByKey(key)
		if !ok || inst.Definition == nil {
			continue
		}
		specs = append(specs, *inst.Definition)
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

var standardSpecKeys = []string{
	"heavy_cream",
	"light_cream",
	"whole_milk",
	"skim_milk",
	"skim_milk_powder",
	"sweetened_condensed_milk",
	"butter",
	"egg_yolk",
	"sucrose",
	"corn_syrup_42",
	"liquid_sugar",
	"cocoa_powder",
	"vanilla_extract",
	"stabilizer",
	"tapioca_syrup",
}
