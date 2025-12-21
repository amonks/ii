package creamery

import "sync"

// IngredientSpec represents an ingredient definition with uncertainty ranges.
type IngredientSpec struct {
	ID      IngredientID
	Name    string
	Profile ConstituentProfile
}

// IngredientLot represents a particular lot of an ingredient, optionally
// overriding metadata such as the display name or constituent profile.
type IngredientLot struct {
	Ingredient IngredientSpec
	Profile    ConstituentProfile
	Name       string
	LotCode    string
}

// Backward-compatible aliases to preserve existing call sites while the
// refactor migrates to the new type names.
type Ingredient = IngredientSpec
type IngredientInstance = IngredientLot

// NewIngredientLot constructs an instance from a base ingredient.
func NewIngredientLot(ing IngredientSpec) IngredientLot {
	profile := ing.Profile
	if profile.ID == "" {
		profile.ID = ing.ID
	}
	if profile.Name == "" {
		profile.Name = ing.Name
	}
	return IngredientLot{
		Ingredient: ing,
		Profile:    profile,
		Name:       profile.Name,
	}
}

// NewIngredientInstance is provided for backward compatibility while the code
// migrates to the new IngredientLot name.
func NewIngredientInstance(ing Ingredient) IngredientInstance {
	return NewIngredientLot(ing)
}

// EffectiveProfile returns the profile for the lot, falling back to the base
// ingredient when no override is provided.
func (inst IngredientLot) EffectiveProfile() ConstituentProfile {
	profile := inst.Profile
	if profile.ID == "" {
		profile.ID = inst.Ingredient.ID
	}
	if profile.Name == "" {
		if inst.Name != "" {
			profile.Name = inst.Name
		} else {
			profile.Name = inst.Ingredient.Name
		}
	}
	if profile.ID == "" {
		profile.ID = NewIngredientID(profile.Name)
	}
	return profile
}

// DisplayName returns the preferred name for the lot.
func (inst IngredientLot) DisplayName() string {
	if inst.Name != "" {
		return inst.Name
	}
	if inst.Ingredient.Name != "" {
		return inst.Ingredient.Name
	}
	return inst.Ingredient.ID.String()
}

// CostPerKg returns the midpoint cost for the lot.
func (inst IngredientLot) CostPerKg() float64 {
	profile := inst.EffectiveProfile()
	cost := profile.Economics.Cost
	if cost.Lo == 0 && cost.Hi == 0 {
		return 0
	}
	return cost.Mid()
}

// IngredientCatalog exposes canonical ingredient specs and their default lots.
type IngredientCatalog struct {
	specs map[IngredientID]IngredientSpec
	lots  map[IngredientID]IngredientLot
	keyed map[string]IngredientLot
}

var (
	defaultCatalog     IngredientCatalog
	defaultCatalogOnce sync.Once
)

// DefaultIngredientCatalog returns the lazily constructed catalog for built-in ingredients.
func DefaultIngredientCatalog() IngredientCatalog {
	defaultCatalogOnce.Do(func() {
		defaultCatalog = catalogFromBatches(IngredientBatchTable())
	})
	return defaultCatalog
}

// NewIngredientCatalog builds a catalog from a slice of ingredient specs. The
// catalog automatically provisions default lots that mirror each spec.
func NewIngredientCatalog(ingredients []Ingredient) IngredientCatalog {
	specs := make(map[IngredientID]Ingredient, len(ingredients))
	lots := make(map[IngredientID]IngredientLot, len(ingredients))
	for _, ing := range ingredients {
		if ing.ID == "" {
			ing.ID = NewIngredientID(ing.Name)
		}
		if existing, ok := specs[ing.ID]; ok && existing.Name != ing.Name {
			if len(ing.Name) > len(existing.Name) {
				specs[ing.ID] = ing
			}
			continue
		}
		specs[ing.ID] = ing
		lots[ing.ID] = NewIngredientLot(ing)
	}
	return IngredientCatalog{
		specs: specs,
		lots:  lots,
		keyed: make(map[string]IngredientLot),
	}
}

// All returns every ingredient in the catalog.
func (c IngredientCatalog) All() []Ingredient {
	all := make([]Ingredient, 0, len(c.specs))
	for _, ing := range c.specs {
		all = append(all, ing)
	}
	return all
}

// Get looks up an ingredient by ID.
func (c IngredientCatalog) Get(id IngredientID) (Ingredient, bool) {
	ing, ok := c.specs[id]
	return ing, ok
}

// Instance returns the default instance for an ingredient ID.
func (c IngredientCatalog) Instance(id IngredientID) (IngredientInstance, bool) {
	inst, ok := c.lots[id]
	return inst, ok
}

// InstanceByKey looks up an instance by its catalog key (e.g., "sucrose").
func (c IngredientCatalog) InstanceByKey(key string) (IngredientInstance, bool) {
	inst, ok := c.keyed[key]
	return inst, ok
}

// Instances returns a copy of the default instances keyed by ingredient ID.
func (c IngredientCatalog) Instances() map[IngredientID]IngredientInstance {
	copy := make(map[IngredientID]IngredientInstance, len(c.lots))
	for id, inst := range c.lots {
		copy[id] = inst
	}
	return copy
}

func catalogFromBatches(batches map[string]IngredientBatch) IngredientCatalog {
	specs := make(map[IngredientID]Ingredient, len(batches))
	lots := make(map[IngredientID]IngredientInstance, len(batches))
	keyed := make(map[string]IngredientInstance, len(batches))
	for key, batch := range batches {
		inst := batch.ToInstance()
		specs[inst.Ingredient.ID] = inst.Ingredient
		lots[inst.Ingredient.ID] = inst
		keyed[key] = inst
	}
	return IngredientCatalog{
		specs: specs,
		lots:  lots,
		keyed: keyed,
	}
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
