package creamery

// IngredientSpec represents an ingredient definition with uncertainty ranges.
type IngredientSpec struct {
	ID        IngredientID
	Name      string
	Profile   ConstituentProfile
	Sweetener SweetenerProps
}

// IngredientLibrary holds reusable specs accessible by ID.
type IngredientLibrary struct {
	Specs map[IngredientID]IngredientSpec
}

// SpecFromProfile builds an IngredientSpec from an existing constituent profile.
func SpecFromProfile(profile ConstituentProfile, sweetener SweetenerProps) IngredientSpec {
	return IngredientSpec{
		ID:        profile.ID,
		Name:      profile.Name,
		Profile:   profile,
		Sweetener: sweetener,
	}
}

// SpecFromComposition constructs a spec from a higher-level composition.
func SpecFromComposition(name string, comp Composition, sweetener SweetenerProps) IngredientSpec {
	profile := ProfileFromComposition(NewIngredientID(name), name, comp)
	return SpecFromProfile(profile, sweetener)
}

// LegacyIngredient converts the spec into the legacy Ingredient type.
func (spec IngredientSpec) LegacyIngredient() Ingredient {
	comp := CompositionFromProfile(spec.Profile)
	return Ingredient{
		Name:      spec.Name,
		Comp:      comp,
		Sweetener: spec.Sweetener,
	}
}

// NewIngredientLibrary builds a library from constituent profiles.
func NewIngredientLibrary(profiles map[IngredientID]ConstituentProfile) IngredientLibrary {
	specs := make(map[IngredientID]IngredientSpec, len(profiles))
	for id, profile := range profiles {
		specs[id] = SpecFromProfile(profile, SweetenerProps{})
	}
	return IngredientLibrary{Specs: specs}
}
