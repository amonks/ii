package creamery

// IngredientProfile wraps a constituent profile with contextual metadata.
type IngredientProfile struct {
	Profile ConstituentProfile
}

// IngredientSpec represents an ingredient definition with uncertainty ranges.
type IngredientSpec struct {
	ID        IngredientID
	Name      string
	Profile   IngredientProfile
	Sweetener SweetenerProps
}

// IngredientBatch represents a concrete ingredient lot with point values.
type IngredientBatch struct {
	ID      IngredientID
	Name    string
	Profile IngredientProfile
}

// IngredientLibrary holds reusable specs accessible by ID.
type IngredientLibrary struct {
	Specs map[IngredientID]IngredientSpec
}

// NewIngredientLibrary builds a library from constituent profiles.
func NewIngredientLibrary(profiles map[IngredientID]ConstituentProfile) IngredientLibrary {
	specs := make(map[IngredientID]IngredientSpec, len(profiles))
	for id, profile := range profiles {
		specs[id] = IngredientSpec{
			ID:      id,
			Name:    profile.Name,
			Profile: IngredientProfile{Profile: profile},
		}
	}
	return IngredientLibrary{Specs: specs}
}
