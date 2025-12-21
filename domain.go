package creamery

import "sync"

// IngredientSpec remains as a convenience alias while the new domain model
// migrates callers to use IngredientDefinition pointers internally.
type IngredientSpec = IngredientDefinition

type IngredientLot = LotDescriptor

// NewIngredientLot constructs a lot descriptor from a spec value.
func NewIngredientLot(spec IngredientSpec) IngredientLot {
	normalized := normalizeSpec(spec)
	definition := normalized
	return NewLot(&definition)
}

// IngredientCatalog exposes canonical ingredient defs and their default lots.
type IngredientCatalog struct {
	defs  map[IngredientID]*IngredientDefinition
	lots  map[IngredientID]IngredientLot
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
func NewIngredientCatalog(ingredients []IngredientSpec) IngredientCatalog {
	defs := make(map[IngredientID]*IngredientDefinition, len(ingredients))
	lots := make(map[IngredientID]IngredientLot, len(ingredients))
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
func (c IngredientCatalog) Instance(id IngredientID) (IngredientLot, bool) {
	lot, ok := c.lots[id]
	return lot, ok
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
	id, ok := c.keyed[normalized]
	if !ok {
		return IngredientLot{}, false
	}
	lot, ok := c.lots[id]
	return lot, ok
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
	specs := make([]IngredientSpec, 0, len(profiles))
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
