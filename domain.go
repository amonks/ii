package creamery

import "sync"

// Backwards compatibility aliases during the migration window.
type IngredientSpec = IngredientDefinition
type IngredientLot = LotDescriptor

// NewIngredientLot constructs an instance from a base ingredient.
// IngredientCatalog exposes canonical ingredient specs and their default lots.
type IngredientCatalog struct {
	specs map[IngredientID]IngredientSpec
	lots  map[IngredientID]IngredientLot
	keyed map[IngredientKey]IngredientLot
}

var (
	defaultCatalog     IngredientCatalog
	defaultCatalogOnce sync.Once
)

// DefaultIngredientCatalog returns the lazily constructed catalog for built-in ingredients.
func DefaultIngredientCatalog() IngredientCatalog {
	defaultCatalogOnce.Do(func() {
		defaultCatalog = catalogFromProfiles(IngredientProfileTable())
	})
	return defaultCatalog
}

// NewIngredientCatalog builds a catalog from a slice of ingredient specs. The
// catalog automatically provisions default lots that mirror each spec.
func NewIngredientCatalog(ingredients []IngredientSpec) IngredientCatalog {
	specs := make(map[IngredientID]IngredientSpec, len(ingredients))
	lots := make(map[IngredientID]IngredientLot, len(ingredients))
	keyed := make(map[IngredientKey]IngredientLot)
	for _, ing := range ingredients {
		ing = normalizeSpec(ing)
		if existing, ok := specs[ing.ID]; ok && existing.Name != ing.Name {
			if len(ing.Name) > len(existing.Name) {
				specs[ing.ID] = ing
				lot := NewIngredientLot(ing)
				lots[ing.ID] = lot
				if ing.Key != "" {
					keyed[ing.Key] = lot
				}
			}
			continue
		}
		specs[ing.ID] = ing
		lot := NewIngredientLot(ing)
		lots[ing.ID] = lot
		if ing.Key != "" {
			keyed[ing.Key] = lot
		}
	}
	return IngredientCatalog{
		specs: specs,
		lots:  lots,
		keyed: keyed,
	}
}

// All returns every ingredient in the catalog.
func (c IngredientCatalog) All() []IngredientSpec {
	all := make([]IngredientSpec, 0, len(c.specs))
	for _, ing := range c.specs {
		all = append(all, ing)
	}
	return all
}

// Get looks up an ingredient by ID.
func (c IngredientCatalog) Get(id IngredientID) (IngredientSpec, bool) {
	ing, ok := c.specs[id]
	return ing, ok
}

// Instance returns the default instance for an ingredient ID.
func (c IngredientCatalog) Instance(id IngredientID) (IngredientLot, bool) {
	inst, ok := c.lots[id]
	return inst, ok
}

// InstanceByKey looks up an instance by its catalog key (e.g., "sucrose").
func (c IngredientCatalog) InstanceByKey(key string) (IngredientLot, bool) {
	if key == "" {
		return IngredientLot{}, false
	}
	normalized := NewIngredientKey(key)
	if normalized == "" {
		return IngredientLot{}, false
	}
	inst, ok := c.keyed[normalized]
	return inst, ok
}

// Instances returns a copy of the default instances keyed by ingredient ID.
func (c IngredientCatalog) Instances() map[IngredientID]IngredientLot {
	copy := make(map[IngredientID]IngredientLot, len(c.lots))
	for id, inst := range c.lots {
		copy[id] = inst
	}
	return copy
}

func catalogFromProfiles(profiles map[string]ConstituentProfile) IngredientCatalog {
	specs := make(map[IngredientID]IngredientSpec, len(profiles))
	lots := make(map[IngredientID]IngredientLot, len(profiles))
	keyed := make(map[IngredientKey]IngredientLot, len(profiles))
	for key, profile := range profiles {
		spec := SpecFromProfile(profile)
		if spec.Key == "" {
			spec.Key = NewIngredientKey(key)
		}
		inst := NewIngredientLot(spec)
		inst.SetProfileOverride(profile)
		specs[inst.Ingredient.ID] = inst.Ingredient
		lots[inst.Ingredient.ID] = inst
		if inst.Ingredient.Key != "" {
			keyed[inst.Ingredient.Key] = inst
		}
	}
	return IngredientCatalog{
		specs: specs,
		lots:  lots,
		keyed: keyed,
	}
}

// SpecFromProfile builds an IngredientSpec from an existing constituent profile.
func SpecFromProfile(profile ConstituentProfile) IngredientSpec {
	return normalizeSpec(IngredientSpec{
		ID:      profile.ID,
		Name:    profile.Name,
		Profile: profile,
	})
}

func normalizeSpec(spec IngredientSpec) IngredientSpec {
	if spec.ID == "" {
		spec.ID = NewIngredientID(spec.Name)
	}
	if spec.Name == "" && spec.ID != "" {
		spec.Name = spec.ID.String()
	}
	if spec.Name == "" {
		spec.Name = "ingredient"
	}
	spec.Key = canonicalIngredientKey(spec.Key, spec.ID)
	spec.Profile = normalizeProfile(spec.Profile, spec.ID, spec.Name)
	return spec
}

func normalizeProfile(profile ConstituentProfile, fallbackID IngredientID, fallbackName string) ConstituentProfile {
	copy := profile
	if copy.ID == "" {
		copy.ID = fallbackID
	}
	if copy.Name == "" {
		copy.Name = fallbackName
	}
	if copy.ID == "" && copy.Name != "" {
		copy.ID = NewIngredientID(copy.Name)
	}
	if copy.Name == "" && copy.ID != "" {
		copy.Name = copy.ID.String()
	}
	if copy.ID == "" {
		copy.ID = IngredientID("ingredient")
	}
	if copy.Name == "" {
		copy.Name = copy.ID.String()
	}
	return copy
}

func canonicalIngredientKey(key IngredientKey, fallback IngredientID) IngredientKey {
	if key != "" {
		return NewIngredientKey(key.String())
	}
	if fallback != "" {
		return IngredientKey(fallback)
	}
	return ""
}

// SpecFromComposition constructs a spec from a higher-level composition.
func SpecFromComposition(name string, comp Composition) IngredientSpec {
	profile := ProfileFromComposition(NewIngredientID(name), name, comp)
	return SpecFromProfile(profile)
}
