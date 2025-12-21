package creamery

// IngredientSpec represents an ingredient definition with uncertainty ranges.
type IngredientSpec struct {
	ID      IngredientID
	Name    string
	Profile ConstituentProfile
}

// IngredientLibrary holds reusable specs accessible by ID.
type IngredientLibrary struct {
	Specs map[IngredientID]IngredientSpec
}

// SpecFromProfile builds an IngredientSpec from an existing constituent profile.
func SpecFromProfile(profile ConstituentProfile) IngredientSpec {
	return IngredientSpec{
		ID:      profile.ID,
		Name:    profile.Name,
		Profile: profile,
	}
}

// SpecFromComposition constructs a spec from a higher-level composition.
func SpecFromComposition(name string, comp Composition) IngredientSpec {
	profile := ProfileFromComposition(NewIngredientID(name), name, comp)
	return SpecFromProfile(profile)
}

// LegacyIngredient converts the spec into the legacy Ingredient type.
func (spec IngredientSpec) LegacyIngredient() Ingredient {
	comp := CompositionFromProfile(spec.Profile)
	legacy := Ingredient{
		ID:   spec.ID,
		Name: spec.Name,
		Comp: comp,
	}
	return canonicalizeIngredient(legacy)
}

// NewIngredientLibrary builds a library from constituent profiles.
func NewIngredientLibrary(profiles map[IngredientID]ConstituentProfile) IngredientLibrary {
	specs := make(map[IngredientID]IngredientSpec, len(profiles))
	for id, profile := range profiles {
		specs[id] = SpecFromProfile(profile)
	}
	return IngredientLibrary{Specs: specs}
}
