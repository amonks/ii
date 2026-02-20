package engine

import "strings"

// SaveBonus represents a conditional modifier to saving throws.
type SaveBonus struct {
	Source      string // where the bonus comes from, e.g. "Moon Sign", "Resilience"
	Description string // short description, e.g. "+2 vs fairy magic and fear"
}

// ConditionalSaveBonuses returns save bonuses granted by kindred traits,
// class traits, and moon sign.
func ConditionalSaveBonuses(kindred, class string, level int, moonSign *MoonSign) []SaveBonus {
	var bonuses []SaveBonus

	// Kindred save bonuses
	for _, t := range KindredTraits(kindred, level) {
		if b, ok := kindredSaveBonus(t); ok {
			bonuses = append(bonuses, b)
		}
	}

	// Class save bonuses
	for _, t := range ClassTraits(class, level) {
		if b, ok := classSaveBonus(t); ok {
			bonuses = append(bonuses, b)
		}
	}

	// Moon sign save bonuses
	if moonSign != nil {
		if b, ok := moonSignSaveBonus(moonSign.Effect); ok {
			bonuses = append(bonuses, b)
		}
	}

	return bonuses
}

func kindredSaveBonus(t Trait) (SaveBonus, bool) {
	switch t.Name {
	case "Resilience":
		return SaveBonus{Source: t.Name, Description: t.Description}, true
	default:
		return SaveBonus{}, false
	}
}

func classSaveBonus(t Trait) (SaveBonus, bool) {
	switch t.Name {
	case "Strength of Will":
		return SaveBonus{Source: t.Name, Description: t.Description}, true
	case "Trophies":
		return SaveBonus{Source: t.Name, Description: t.Description}, true
	default:
		return SaveBonus{}, false
	}
}

func moonSignSaveBonus(effect string) (SaveBonus, bool) {
	lower := strings.ToLower(effect)

	// Check for save bonus/penalty mentions
	if strings.Contains(lower, "saving throw") {
		// Some effects mention saves as part of a larger effect (e.g. Narrow moon Waxing).
		// Extract just the save-related part.
		desc := extractSavePart(effect)
		return SaveBonus{Source: "Moon Sign", Description: desc}, true
	}

	return SaveBonus{}, false
}

// extractSavePart extracts the save-related clause from a moon sign effect.
// If the whole effect is about saves, returns it as-is.
// If saves are mentioned as part of a compound effect, extracts just that part.
func extractSavePart(effect string) string {
	// Handle compound effects joined by ", but " or "; "
	// e.g. "+1 reaction bonus ..., but suffer a -1 penalty to all saving throws against fairy magic."
	if idx := strings.Index(effect, "suffer a "); idx >= 0 {
		part := effect[idx+len("suffer a "):]
		// Capitalize and return
		return part
	}
	return effect
}
