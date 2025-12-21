package creamery

// Ingredient represents an ingredient definition with uncertainty ranges.
type Ingredient struct {
	ID      IngredientID
	Name    string
	Profile ConstituentProfile
}

// IngredientInstance represents a particular lot of an ingredient, optionally
// overriding metadata such as the display name or constituent profile.
type IngredientInstance struct {
	Base       Ingredient
	LotCode    string
	Name       string
	Profile    ConstituentProfile
	CostPerKg  Interval
	LastUpdate string
}

// EffectiveProfile returns the profile for the instance, falling back to the
// base ingredient when no override is provided.
func (inst IngredientInstance) EffectiveProfile() ConstituentProfile {
	if inst.Profile.ID == "" && inst.Profile.Name == "" {
		return inst.Base.Profile
	}
	profile := inst.Profile
	if profile.ID == "" {
		profile.ID = inst.Base.Profile.ID
	}
	if profile.Name == "" {
		profile.Name = inst.Name
		if profile.Name == "" {
			profile.Name = inst.Base.Profile.Name
		}
	}
	return profile
}

// IngredientCatalog holds reusable ingredients accessible by ID.
type IngredientCatalog struct {
	items map[IngredientID]Ingredient
}

// NewIngredientCatalog builds a catalog from a slice of ingredients.
func NewIngredientCatalog(ingredients []Ingredient) IngredientCatalog {
	items := make(map[IngredientID]Ingredient, len(ingredients))
	for _, ing := range ingredients {
		if ing.ID == "" {
			ing.ID = NewIngredientID(ing.Name)
		}
		if existing, ok := items[ing.ID]; ok && existing.Name != ing.Name {
			// Prefer the more descriptive name if duplicate IDs occur.
			if len(ing.Name) > len(existing.Name) {
				items[ing.ID] = ing
			}
			continue
		}
		items[ing.ID] = ing
	}
	return IngredientCatalog{items: items}
}

// All returns every ingredient in the catalog.
func (c IngredientCatalog) All() []Ingredient {
	all := make([]Ingredient, 0, len(c.items))
	for _, ing := range c.items {
		all = append(all, ing)
	}
	return all
}

// Get looks up an ingredient by ID.
func (c IngredientCatalog) Get(id IngredientID) (Ingredient, bool) {
	ing, ok := c.items[id]
	return ing, ok
}

// SpecFromProfile builds an Ingredient from an existing constituent profile.
func SpecFromProfile(profile ConstituentProfile) Ingredient {
	return Ingredient{
		ID:      profile.ID,
		Name:    profile.Name,
		Profile: profile,
	}
}

// SpecFromComposition constructs a spec from a higher-level composition.
func SpecFromComposition(name string, comp Composition) Ingredient {
	profile := ProfileFromComposition(NewIngredientID(name), name, comp)
	return SpecFromProfile(profile)
}
