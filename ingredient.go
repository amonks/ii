package creamery

import "fmt"

// specFromCatalog resolves a catalog key to its canonical spec, returning an
// error when the key is unknown or lacks definition metadata.
func specFromCatalog(key string) (IngredientDefinition, error) {
	inst, ok := DefaultIngredientCatalog().InstanceByKey(key)
	if !ok || inst.Definition == nil {
		return IngredientDefinition{}, fmt.Errorf("ingredient %q not found in catalog", key)
	}
	return *inst.Definition, nil
}

func mustSpecFromCatalog(key string) IngredientDefinition {
	spec, err := specFromCatalog(key)
	if err != nil {
		panic(err)
	}
	return spec
}

// Standard ingredient specifications with typical compositions.
var (
	HeavyCream = mustSpecFromCatalog("heavy_cream")
	LightCream = mustSpecFromCatalog("light_cream")
	WholeMilk  = mustSpecFromCatalog("whole_milk")
	SkimMilk   = mustSpecFromCatalog("skim_milk")

	NonfatDryMilk          = mustSpecFromCatalog("skim_milk_powder")
	SweetenedCondensedMilk = mustSpecFromCatalog("sweetened_condensed_milk")
	Butter                 = mustSpecFromCatalog("butter")
	EggYolks               = mustSpecFromCatalog("egg_yolk")
	Sugar                  = mustSpecFromCatalog("sucrose")
	CornSyrup              = mustSpecFromCatalog("corn_syrup_42")
	LiquidSugar            = mustSpecFromCatalog("liquid_sugar")
	CocoaPowder            = mustSpecFromCatalog("cocoa_powder")
	VanillaExtract         = mustSpecFromCatalog("vanilla_extract")
	Stabilizer             = mustSpecFromCatalog("stabilizer")
	TapiocaSyrup           = mustSpecFromCatalog("tapioca_syrup")

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
