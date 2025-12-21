package creamery

import "sync"

// NewLotDescriptor constructs a lot descriptor from a spec value.
func NewLotDescriptor(spec IngredientDefinition) LotDescriptor {
	normalized := normalizeSpec(spec)
	definition := normalized
	return NewLot(&definition)
}

// IngredientCatalog exposes canonical ingredient defs and their default lots.
type IngredientCatalog struct {
	defs  map[IngredientID]*IngredientDefinition
	lots  map[IngredientID]LotDescriptor
	keyed map[IngredientKey]IngredientID
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
func NewIngredientCatalog(ingredients []IngredientDefinition) IngredientCatalog {
	defs := make(map[IngredientID]*IngredientDefinition, len(ingredients))
	lots := make(map[IngredientID]LotDescriptor, len(ingredients))
	keyed := make(map[IngredientKey]IngredientID)

	for _, ing := range ingredients {
		normalized := normalizeSpec(ing)
		definition := normalized
		if existing, ok := defs[definition.ID]; ok {
			if len(definition.Name) <= len(existing.Name) {
				continue
			}
		}
		defs[definition.ID] = &definition
	}

	for id, def := range defs {
		lot := NewLot(def)
		lots[id] = lot
		if def.Key != "" {
			keyed[def.Key] = id
		}
	}

	return IngredientCatalog{
		defs:  defs,
		lots:  lots,
		keyed: keyed,
	}
}

// All returns every ingredient definition in the catalog.
func (c IngredientCatalog) All() []*IngredientDefinition {
	all := make([]*IngredientDefinition, 0, len(c.defs))
	for _, def := range c.defs {
		all = append(all, def)
	}
	return all
}

// Get looks up an ingredient definition by ID.
func (c IngredientCatalog) Get(id IngredientID) (*IngredientDefinition, bool) {
	def, ok := c.defs[id]
	return def, ok
}

// Instance returns the default lot for an ingredient ID.
func (c IngredientCatalog) Instance(id IngredientID) (LotDescriptor, bool) {
	lot, ok := c.lots[id]
	return lot, ok
}

// InstanceByKey looks up an instance by its catalog key (e.g., "sucrose").
func (c IngredientCatalog) InstanceByKey(key string) (LotDescriptor, bool) {
	if key == "" {
		return LotDescriptor{}, false
	}
	normalized := NewIngredientKey(key)
	if normalized == "" {
		return LotDescriptor{}, false
	}
	id, ok := c.keyed[normalized]
	if !ok {
		return LotDescriptor{}, false
	}
	lot, ok := c.lots[id]
	return lot, ok
}

// Instances returns a copy of the default instances keyed by ingredient ID.
func (c IngredientCatalog) Instances() map[IngredientID]LotDescriptor {
	copy := make(map[IngredientID]LotDescriptor, len(c.lots))
	for id, inst := range c.lots {
		copy[id] = inst
	}
	return copy
}

func catalogFromProfiles(profiles map[string]ConstituentProfile) IngredientCatalog {
	specs := make([]IngredientDefinition, 0, len(profiles))
	overrides := make(map[IngredientID]ConstituentProfile, len(profiles))
	for key, profile := range profiles {
		spec := SpecFromProfile(profile)
		if spec.Key == "" {
			spec.Key = NewIngredientKey(key)
		}
		specs = append(specs, spec)
		overrides[spec.ID] = profile
	}

	catalog := NewIngredientCatalog(specs)
	for id, override := range overrides {
		lot, ok := catalog.lots[id]
		if !ok {
			continue
		}
		lot.SetProfileOverride(override)
		catalog.lots[id] = lot
	}
	return catalog
}

// SpecFromProfile builds an IngredientDefinition from an existing constituent profile.
func SpecFromProfile(profile ConstituentProfile) IngredientDefinition {
	return normalizeSpec(IngredientDefinition{
		ID:      profile.ID,
		Name:    profile.Name,
		Profile: profile,
	})
}

func normalizeSpec(spec IngredientDefinition) IngredientDefinition {
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
func SpecFromComposition(name string, comp Composition) IngredientDefinition {
	profile := ProfileFromComposition(NewIngredientID(name), name, comp)
	return SpecFromProfile(profile)
}
