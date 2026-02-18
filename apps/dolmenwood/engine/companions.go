package engine

// CompanionStats holds the default stats for a horse breed.
type CompanionStats struct {
	AC           int
	HPMax        int
	Speed        int
	LoadCapacity int // in slots (coins / 100)
}

var breeds = map[string]CompanionStats{
	"Charger":          {AC: 12, HPMax: 13, Speed: 40, LoadCapacity: 40},
	"Dapple-doff":      {AC: 12, HPMax: 13, Speed: 30, LoadCapacity: 50},
	"Hop-clopper":      {AC: 12, HPMax: 13, Speed: 30, LoadCapacity: 50},
	"Mule":             {AC: 12, HPMax: 9, Speed: 40, LoadCapacity: 25},
	"Prigwort prancer": {AC: 12, HPMax: 9, Speed: 80, LoadCapacity: 30},
	"Yellow-flank":     {AC: 12, HPMax: 13, Speed: 60, LoadCapacity: 35},
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
