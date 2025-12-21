package creamery

import "fmt"

// Ingredient represents a component that can be used in ice cream.
type Ingredient struct {
	ID      IngredientID
	Name    string
	Comp    Composition
	Profile ConstituentProfile
}

// String returns a human-readable representation.
func (i Ingredient) String() string {
	return fmt.Sprintf("%s: %s", i.Name, i.Comp)
}

func canonicalizeIngredient(ing Ingredient) Ingredient {
	if ing.ID == "" {
		ing.ID = NewIngredientID(ing.Name)
	}
	if ing.Name == "" {
		ing.Name = ing.ID.String()
	}
	if ing.Profile.ID == "" && ing.Profile.Name == "" {
		ing.Profile = ProfileFromComposition(ing.ID, ing.Name, ing.Comp)
	} else {
		if ing.Profile.ID == "" {
			ing.Profile.ID = ing.ID
		}
		if ing.Profile.Name == "" {
			ing.Profile.Name = ing.Name
		}
	}
	// Keep Composition in sync with profile if missing.
	if isZeroComposition(ing.Comp) {
		ing.Comp = CompositionFromProfile(ing.Profile)
	}
	return ing
}

func isZeroComposition(c Composition) bool {
	return c.Fat.Lo == 0 && c.Fat.Hi == 0 &&
		c.MSNF.Lo == 0 && c.MSNF.Hi == 0 &&
		c.Sugar.Lo == 0 && c.Sugar.Hi == 0 &&
		c.Other.Lo == 0 && c.Other.Hi == 0
}

func legacyIngredientFromBatch(key, displayName string) Ingredient {
	batch, ok := IngredientBatchTable()[key]
	if !ok {
		return Ingredient{Name: displayName}
	}
	if displayName != "" {
		batch.Name = displayName
	}
	return batch.ToSpec().LegacyIngredient()
}

// Common dairy ingredients with typical composition ranges.
// Sources: USDA, dairy science literature.
var (
	// Heavy cream: 36-40% fat, ~2% protein, ~3% lactose
	HeavyCream = Ingredient{
		Name: "Heavy Cream",
		Comp: Composition{
			Fat:   Range(0.36, 0.40),
			MSNF:  Range(0.05, 0.06), // protein + lactose + minerals
			Sugar: Point(0),
			Other: Point(0),
		},
	}

	// Light cream: 18-30% fat
	LightCream = Ingredient{
		Name: "Light Cream",
		Comp: Composition{
			Fat:   Range(0.18, 0.30),
			MSNF:  Range(0.06, 0.08),
			Sugar: Point(0),
			Other: Point(0),
		},
	}

	// Whole milk: ~3.25% fat, ~8.5% MSNF
	WholeMilk = Ingredient{
		Name: "Whole Milk",
		Comp: Composition{
			Fat:   Range(0.032, 0.035),
			MSNF:  Range(0.085, 0.09),
			Sugar: Point(0),
			Other: Point(0),
		},
	}

	// Skim milk: <0.5% fat, ~9% MSNF
	SkimMilk = Ingredient{
		Name: "Skim Milk",
		Comp: Composition{
			Fat:   Range(0, 0.005),
			MSNF:  Range(0.09, 0.095),
			Sugar: Point(0),
			Other: Point(0),
		},
	}

	// Nonfat dry milk (NFDM): concentrated MSNF
	NonfatDryMilk = Ingredient{
		Name: "Nonfat Dry Milk",
		Comp: Composition{
			Fat:   Range(0.005, 0.015),
			MSNF:  Range(0.95, 0.97), // mostly lactose and protein
			Sugar: Point(0),
			Other: Point(0),
		},
	}

	// Sweetened condensed milk: ~8% fat, ~20% MSNF, ~45% sugar (sucrose)
	SweetenedCondensedMilk = Ingredient{
		Name: "Sweetened Condensed Milk",
		Comp: Composition{
			Fat:   Range(0.08, 0.09),
			MSNF:  Range(0.19, 0.21),
			Sugar: Range(0.43, 0.47),
			Other: Point(0),
		},
	}

	// Butter: ~80% fat, ~2% MSNF
	Butter = Ingredient{
		Name: "Butter",
		Comp: Composition{
			Fat:   Range(0.80, 0.82),
			MSNF:  Range(0.01, 0.02),
			Sugar: Point(0),
			Other: Point(0),
		},
	}

	// Egg yolks: ~30% fat, no MSNF, but significant other solids
	EggYolks = Ingredient{
		Name: "Egg Yolks",
		Comp: Composition{
			Fat:   Range(0.30, 0.33),
			MSNF:  Point(0),
			Sugar: Point(0),
			Other: Range(0.16, 0.18), // protein, lecithin
		},
	}

	// Granulated sugar (sucrose): 100% sugar
	Sugar = Ingredient{
		Name: "Sugar",
		Comp: Composition{
			Fat:   Point(0),
			MSNF:  Point(0),
			Sugar: Point(1.0),
			Other: Point(0),
		},
	}

	// Corn syrup (42 DE): mostly glucose/maltose
	CornSyrup = legacyIngredientFromBatch("corn_syrup_42", "Corn Syrup")

	// Liquid sugar (sucrose + water, typically 67% sugar)
	LiquidSugar = Ingredient{
		Name: "Liquid Sugar",
		Comp: Composition{
			Fat:   Point(0),
			MSNF:  Point(0),
			Sugar: Range(0.65, 0.68),
			Other: Point(0),
		},
	}

	// Cocoa powder: mostly other solids, some fat
	CocoaPowder = Ingredient{
		Name: "Cocoa Powder",
		Comp: Composition{
			Fat:   Range(0.10, 0.24), // varies by type
			MSNF:  Point(0),
			Sugar: Point(0),
			Other: Range(0.70, 0.85), // cocoa solids
		},
	}

	// Vanilla extract: mostly water/alcohol, negligible solids
	VanillaExtract = Ingredient{
		Name: "Vanilla Extract",
		Comp: Composition{
			Fat:   Point(0),
			MSNF:  Point(0),
			Sugar: Point(0),
			Other: Range(0, 0.02),
		},
	}

	// Stabilizer blend (guar, locust bean, carrageenan): pure other
	Stabilizer = Ingredient{
		Name: "Stabilizer",
		Comp: Composition{
			Fat:   Point(0),
			MSNF:  Point(0),
			Sugar: Point(0),
			Other: Point(1.0),
		},
	}

	// Tapioca syrup: similar to corn syrup, mostly glucose/maltodextrin
	TapiocaSyrup = legacyIngredientFromBatch("tapioca_syrup", "Tapioca Syrup")

	// Nonfat Milk (variable concentration): could be liquid skim, powder,
	// or reconstituted to any concentration. MSNF ranges from ~9% (liquid)
	// to ~97% (powder). Use this when label says "Nonfat Milk" without
	// specifying form - let the solver determine required concentration.
	NonfatMilkVariable = Ingredient{
		Name: "Nonfat Milk",
		Comp: Composition{
			Fat:   Range(0, 0.005),   // essentially fat-free
			MSNF:  Range(0.09, 0.97), // liquid skim to dry powder
			Sugar: Point(0),
			Other: Point(0),
		},
	}
)

// StandardLibrary returns a map of common ice cream ingredients.
func StandardLibrary() map[string]Ingredient {
	ingredients := []Ingredient{
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
	}
	lib := make(map[string]Ingredient, len(ingredients))
	for _, ing := range ingredients {
		lib[ing.Name] = ing
	}
	return lib
}

// StandardProfiles converts the standard ingredient library to constituent profiles.
func StandardProfiles() map[IngredientID]ConstituentProfile {
	lib := StandardLibrary()
	profiles := make(map[IngredientID]ConstituentProfile, len(lib))
	for _, ing := range lib {
		profile := ing.ToProfile()
		profiles[profile.ID] = profile
	}
	return profiles
}

// StandardSpecs returns ingredient specs for the common ingredient library.
func StandardSpecs() map[IngredientID]IngredientSpec {
	lib := StandardLibrary()
	specs := make(map[IngredientID]IngredientSpec, len(lib))
	for _, ing := range lib {
		spec := SpecFromComposition(ing.Name, ing.Comp)
		specs[spec.ID] = spec
	}
	return specs
}
