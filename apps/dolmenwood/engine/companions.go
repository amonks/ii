package engine

import "strings"

// CompanionStats holds the full stats for a horse/mule breed.
type CompanionStats struct {
	AC           int
	HPMax        int
	Speed        int
	LoadCapacity int // in slots (coins / 100)
	Level        int
	Saves        SaveTargets
	Attack       string
	Morale       int
}

var breeds = map[string]CompanionStats{
	"Charger":          {AC: 12, HPMax: 13, Speed: 40, LoadCapacity: 40, Level: 3, Saves: SaveTargets{11, 12, 13, 14, 15}, Attack: "2 hooves (+2, 1d6)", Morale: 9},
	"Dapple-doff":      {AC: 12, HPMax: 13, Speed: 30, LoadCapacity: 50, Level: 3, Saves: SaveTargets{11, 12, 13, 14, 15}, Attack: "None", Morale: 5},
	"Hop-clopper":      {AC: 12, HPMax: 13, Speed: 30, LoadCapacity: 50, Level: 3, Saves: SaveTargets{11, 12, 13, 14, 15}, Attack: "2 hooves (+2, 1d4)", Morale: 7},
	"Mule":             {AC: 12, HPMax: 9, Speed: 40, LoadCapacity: 25, Level: 2, Saves: SaveTargets{12, 13, 14, 15, 16}, Attack: "Kick (+1, 1d4) or bite (+1, 1d3)", Morale: 8},
	"Prigwort prancer": {AC: 12, HPMax: 9, Speed: 80, LoadCapacity: 30, Level: 2, Saves: SaveTargets{12, 13, 14, 15, 16}, Attack: "2 hooves (+1, 1d4)", Morale: 7},
	"Yellow-flank":     {AC: 12, HPMax: 13, Speed: 60, LoadCapacity: 35, Level: 3, Saves: SaveTargets{11, 12, 13, 14, 15}, Attack: "2 hooves (+2, 1d4)", Morale: 7},
}

// breedOrder preserves a consistent display order.
var breedOrder = []string{
	"Charger",
	"Dapple-doff",
	"Hop-clopper",
	"Mule",
	"Prigwort prancer",
	"Yellow-flank",
}

// BreedStats returns the default stats for a named breed.
func BreedStats(breed string) (CompanionStats, bool) {
	s, ok := breeds[breed]
	return s, ok
}

// BreedNames returns all known breed names in display order.
func BreedNames() []string {
	return breedOrder
}

// IsCompanionBreed returns true if the name matches a known horse/mule breed.
func IsCompanionBreed(name string) bool {
	for breed := range breeds {
		if strings.EqualFold(breed, name) {
			return true
		}
	}
	return false
}

// IsCompanionGear returns true if the item is companion gear (saddle/bridle/barding)
// that enables companion capacity rather than consuming it.
func IsCompanionGear(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "pack saddle and bridle", "riding saddle and bridle", "horse barding":
		return true
	}
	return false
}

// CompanionSaddleTypeFromItems scans items for a saddle and returns "pack", "riding", or "".
func CompanionSaddleTypeFromItems(items []Item) string {
	for _, item := range items {
		lower := strings.ToLower(item.Name)
		switch lower {
		case "pack saddle and bridle":
			return "pack"
		case "riding saddle and bridle":
			return "riding"
		}
	}
	return ""
}

// CompanionHasBardingFromItems scans items for horse barding.
func CompanionHasBardingFromItems(items []Item) bool {
	for _, item := range items {
		if strings.EqualFold(item.Name, "horse barding") {
			return true
		}
	}
	return false
}

// CompanionAC returns the effective AC for a companion.
// Barding grants +2 AC.
func CompanionAC(baseAC int, hasBarding bool) int {
	if hasBarding {
		return baseAC + 2
	}
	return baseAC
}

// CompanionLoadCapacity returns the effective load capacity based on saddle type.
// No saddle: 0 slots. Riding: saddlebags only (5 slots). Pack: full breed capacity.
func CompanionLoadCapacity(breedCapacity int, saddleType string) int {
	switch saddleType {
	case "riding":
		return 5 // saddlebags capacity
	case "pack":
		return breedCapacity
	default:
		return 0
	}
}
