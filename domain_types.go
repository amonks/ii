package creamery

import "strings"

// Ingredient is the canonical immutable ingredient specification shared across
// the catalog, solver, and recipe layers. It encapsulates the normalized
// constituent profile plus optional catalog metadata such as a stable key.
type Ingredient struct {
	ID      IngredientID
	Key     IngredientKey
	Name    string
	Profile ConstituentProfile
}

// Lot couples an ingredient definition with optional lot metadata such as
// display name overrides and constituent overrides.
type Lot struct {
	Definition      *Ingredient
	Label           string
	LotCode         string
	profileOverride *ConstituentProfile
}

// NewIngredient normalizes the provided profile metadata and returns an
// immutable ingredient value.
func NewIngredient(profile ConstituentProfile, key IngredientKey) Ingredient {
	ingredient := Ingredient{
		ID:      profile.ID,
		Key:     key,
		Name:    profile.Name,
		Profile: profile,
	}
	return normalizeIngredient(ingredient)
}


// NewLot creates a lot descriptor for the provided definition.
func NewLot(def *Ingredient) Lot {
	if def == nil {
		return Lot{}
	}
	return Lot{
		Definition: def,
		Label:      def.Name,
	}
}

// EffectiveProfile returns the constituent profile for the lot, applying any
// overrides and ensuring IDs/names remain aligned with the definition.
func (lot Lot) EffectiveProfile() ConstituentProfile {
	if lot.Definition == nil {
		return normalizeProfile(ConstituentProfile{}, "", lot.Label)
	}
	profile := lot.Definition.Profile
	if lot.profileOverride != nil {
		profile = normalizeProfile(*lot.profileOverride, lot.Definition.ID, lot.displayName())
	} else if profile.Name == "" && lot.Label != "" {
		profile.Name = lot.Label
	}
	return profile
}

// DisplayName exposes the preferred name for the lot.
func (lot Lot) DisplayName() string {
	return lot.displayName()
}

func (lot Lot) displayName() string {
	if lot.Label != "" {
		return lot.Label
	}
	if lot.Definition != nil && lot.Definition.Name != "" {
		return lot.Definition.Name
	}
	if lot.Definition != nil {
		return lot.Definition.ID.String()
	}
	return ""
}

// SetProfileOverride replaces the lot's constituent profile while keeping
// definition metadata intact.
func (lot *Lot) SetProfileOverride(profile ConstituentProfile) {
	if lot == nil || lot.Definition == nil {
		return
	}
	normalized := normalizeProfile(profile, lot.Definition.ID, lot.displayName())
	lot.profileOverride = &normalized
}

// WithProfileOverride returns a copy of the lot with a different profile.
func (lot Lot) WithProfileOverride(profile ConstituentProfile) Lot {
	lot.SetProfileOverride(profile)
	return lot
}

// WithDefinition returns a copy of the lot using the provided definition.
func (lot Lot) WithDefinition(def *Ingredient) Lot {
	if def == nil {
		return lot
	}
	lot.Definition = def
	if lot.Label == "" || lot.Label == def.Name {
		lot.Label = def.Name
	}
	lot.profileOverride = nil
	return lot
}

// WithSpec returns a copy of the lot backed by the provided spec value.
func (lot Lot) WithSpec(spec Ingredient) Lot {
	normalized := normalizeSpec(spec)
	definition := normalized
	lot.Definition = &definition
	if lot.Label == "" || lot.Label == definition.Name {
		lot.Label = definition.Name
	}
	lot.profileOverride = nil
	return lot
}

// CostPerKg returns the midpoint cost for the lot's effective profile.
func (lot Lot) CostPerKg() float64 {
	profile := lot.EffectiveProfile()
	cost := profile.Economics.Cost
	if cost.Lo == 0 && cost.Hi == 0 {
		return 0
	}
	return cost.Mid()
}

// normalizeIngredient enforces ID/key/name/profile invariants on definitions.
func normalizeIngredient(def Ingredient) Ingredient {
	def.Name = strings.TrimSpace(def.Name)
	if def.ID == "" {
		def.ID = NewIngredientID(def.Name)
	}
	if def.Name == "" && def.ID != "" {
		def.Name = def.ID.String()
	}
	if def.Name == "" {
		def.Name = "ingredient"
	}
	def.Key = canonicalIngredientKey(def.Key, def.ID)
	def.Profile = normalizeProfile(def.Profile, def.ID, def.Name)
	return def
}

// DefaultLot returns a lot descriptor whose profile matches the definition.
func (def Ingredient) DefaultLot() Lot {
	normalized := normalizeIngredient(def)
	copy := normalized
	return NewLot(&copy)
}

// Legacy helper retained for compatibility while call sites migrate to the new
// normalizeIngredient name.
func normalizeDefinition(def Ingredient) Ingredient {
	return normalizeIngredient(def)
}
