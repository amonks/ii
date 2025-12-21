package creamery

import "strings"

// IngredientDefinition is the canonical immutable ingredient specification shared
// across the catalog, solver, and recipe layers.
type IngredientDefinition struct {
	ID      IngredientID
	Key     IngredientKey
	Name    string
	Profile ConstituentProfile
}

// LotDescriptor couples an ingredient definition with optional lot metadata
// such as display name overrides and constituent overrides.
type LotDescriptor struct {
	Definition      *IngredientDefinition
	Label           string
	LotCode         string
	profileOverride *ConstituentProfile
}

// NewIngredientDefinition normalizes the provided profile metadata and returns
// an immutable definition pointer.
func NewIngredientDefinition(profile ConstituentProfile, key IngredientKey) *IngredientDefinition {
	definition := IngredientDefinition{
		ID:      profile.ID,
		Key:     key,
		Name:    profile.Name,
		Profile: profile,
	}
	normalized := normalizeDefinition(definition)
	return &normalized
}

// NewLot creates a lot descriptor for the provided definition.
func NewLot(def *IngredientDefinition) LotDescriptor {
	if def == nil {
		return LotDescriptor{}
	}
	return LotDescriptor{
		Definition: def,
		Label:      def.Name,
	}
}

// EffectiveProfile returns the constituent profile for the lot, applying any
// overrides and ensuring IDs/names remain aligned with the definition.
func (lot LotDescriptor) EffectiveProfile() ConstituentProfile {
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
func (lot LotDescriptor) DisplayName() string {
	return lot.displayName()
}

func (lot LotDescriptor) displayName() string {
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
func (lot *LotDescriptor) SetProfileOverride(profile ConstituentProfile) {
	if lot == nil || lot.Definition == nil {
		return
	}
	normalized := normalizeProfile(profile, lot.Definition.ID, lot.displayName())
	lot.profileOverride = &normalized
}

// WithProfileOverride returns a copy of the lot with a different profile.
func (lot LotDescriptor) WithProfileOverride(profile ConstituentProfile) LotDescriptor {
	lot.SetProfileOverride(profile)
	return lot
}

// WithDefinition returns a copy of the lot using the provided definition.
func (lot LotDescriptor) WithDefinition(def *IngredientDefinition) LotDescriptor {
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
func (lot LotDescriptor) WithSpec(spec IngredientDefinition) LotDescriptor {
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
func (lot LotDescriptor) CostPerKg() float64 {
	profile := lot.EffectiveProfile()
	cost := profile.Economics.Cost
	if cost.Lo == 0 && cost.Hi == 0 {
		return 0
	}
	return cost.Mid()
}

// normalizeDefinition enforces ID/key/name/profile invariants on definitions.
func normalizeDefinition(def IngredientDefinition) IngredientDefinition {
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
func (def IngredientDefinition) DefaultLot() LotDescriptor {
	normalized := normalizeDefinition(def)
	copy := normalized
	return NewLot(&copy)
}
