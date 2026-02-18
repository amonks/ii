package engine

import "strings"

// Weapon holds stats for a known weapon.
type Weapon struct {
	Damage    string
	Qualities string
	Weight    int // in coins
}

// Armor holds stats for known armor.
type Armor struct {
	AC     int
	Weight int // in coins
}

var weapons = map[string]Weapon{
	"battle axe":       {Damage: "1d8", Qualities: "Melee", Weight: 100},
	"club":             {Damage: "1d4", Qualities: "Melee", Weight: 20},
	"crossbow":         {Damage: "1d8", Qualities: "Missile, Reload, Two-handed", Weight: 50},
	"dagger":           {Damage: "1d4", Qualities: "Melee, Missile", Weight: 10},
	"hand axe":         {Damage: "1d6", Qualities: "Melee, Missile", Weight: 20},
	"lance":            {Damage: "1d6", Qualities: "Brace, Charge, Melee, Reach", Weight: 100},
	"longbow":          {Damage: "1d6", Qualities: "Missile, Two-handed", Weight: 40},
	"longsword":        {Damage: "1d8", Qualities: "Melee", Weight: 30},
	"mace":             {Damage: "1d6", Qualities: "Melee", Weight: 40},
	"polearm":          {Damage: "1d10", Qualities: "Brace, Melee, Reach, Two-handed", Weight: 140},
	"shortbow":         {Damage: "1d6", Qualities: "Missile, Two-handed", Weight: 20},
	"shortsword":       {Damage: "1d6", Qualities: "Melee", Weight: 20},
	"sling":            {Damage: "1d4", Qualities: "Missile", Weight: 10},
	"spear":            {Damage: "1d6", Qualities: "Brace, Melee, Missile", Weight: 30},
	"staff":            {Damage: "1d4", Qualities: "Melee, Two-handed", Weight: 40},
	"two-handed sword": {Damage: "1d10", Qualities: "Melee, Two-handed", Weight: 140},
	"war hammer":       {Damage: "1d6", Qualities: "Melee", Weight: 40},
}

var armors = map[string]Armor{
	"leather":       {AC: 12, Weight: 200},
	"bark":          {AC: 13, Weight: 300},
	"chainmail":     {AC: 14, Weight: 400},
	"chain mail":    {AC: 14, Weight: 400},
	"pinecone":      {AC: 15, Weight: 400},
	"plate mail":    {AC: 16, Weight: 500},
	"full plate":    {AC: 17, Weight: 700},
	"shield":        {AC: 1, Weight: 100},
}

// itemWeights maps known item names to their weight in coins per unit.
// From the Dolmenwood adventuring gear tables.
var itemWeights = map[string]int{
	// Containers
	"backpack":               50,
	"barrel":                 70,
	"belt pouch":             10,
	"bucket":                 20,
	"casket (iron, large)":   400,
	"casket (iron, small)":   100,
	"chest (wooden, large)":  200,
	"chest (wooden, small)":  50,
	"sack":                   5,
	"scroll case":            5,
	"vial":                   1,
	"waterskin":              50,

	// Light
	"candles":            2,  // per candle (sold as 10)
	"lantern":            20,
	"lantern (hooded)":   20,
	"lantern (bullseye)": 20,
	"oil flask":          10,
	"oil":                10,
	"tinder box":         10,
	"torch":              10, // per torch (sold as 3)
	"torches":            10,

	// Camping and Travel
	"bedroll":               70,
	"cooking pots":          100,
	"firewood":              200,
	"fishing rod":           50,
	"fishing rod and tackle": 50,
	"preserved rations":     20,
	"rations (preserved)":   20,
	"fresh rations":         20,
	"rations (fresh)":       20,
	"rations":               20,
	"tent":                  20,

	// Holy Items
	"holy symbol (gold)":   20,
	"holy symbol (silver)": 20,
	"holy symbol (wooden)": 10,
	"holy symbol":          10,
	"holy water":           10,

	// Ammunition
	"arrows":       20,
	"quarrels":     20,
	"sling stones": 1,

	// Miscellaneous Tools
	"bell":              1,
	"block and tackle":  50,
	"caltrops":          1,  // per caltrop
	"chain":             100,
	"chalk":             1,  // per stick
	"chisel":            20,
	"crowbar":           50,
	"grappling hook":    40,
	"hammer":            30,
	"hammer (small)":    30,
	"sledgehammer":      100,
	"ink":               5,
	"iron spikes":       5,  // per spike
	"lock":              10,
	"magnifying glass":  5,
	"manacles":          60,
	"marbles":           1,  // per marble
	"mining pick":       100,
	"mirror":            50,
	"mirror (small)":    50,
	"musical instrument": 50,
	"paper":             0,
	"parchment":         0,
	"pole":              70,
	"quill":             1,
	"rope":              100,
	"rope ladder":       200,
	"saw":               20,
	"shovel":            50,
	"spell book":        50,
	"thieves' tools":    10,
	"twine":             10,
	"whistle":           1,

	// Clothing
	"clothes":       30,
	"winter cloak":  20,
	"robes":         30,

	// Horse accessories
	"feed":                     100,
	"horse barding":            600,
	"pack saddle and bridle":   150,
	"riding saddle and bridle": 300,
	"riding saddle bags":       100,
}

// Container holds the stowed slot capacity for a known container.
type Container struct {
	Slots int
}

var containers = map[string]Container{
	"backpack":   {Slots: 10},
	"sack":       {Slots: 10},
	"belt pouch": {Slots: 1},
}

// ContainerCapacity returns the stowed slot capacity for a known container (case-insensitive).
func ContainerCapacity(name string) (int, bool) {
	c, ok := containers[strings.ToLower(name)]
	return c.Slots, ok
}

// IsContainer returns whether the item name is a known container.
func IsContainer(name string) bool {
	_, ok := containers[strings.ToLower(name)]
	return ok
}

// EquippedWeapon holds stats for an equipped weapon, including the display name.
type EquippedWeapon struct {
	Name      string
	Damage    string
	Qualities string
}

// WeaponStats returns stats for a weapon by name (case-insensitive).
func WeaponStats(name string) (Weapon, bool) {
	w, ok := weapons[strings.ToLower(name)]
	return w, ok
}

// ACFromEquippedItems computes armor class from equipped items and DEX score.
// Returns the AC and the name of the armor (empty string if unarmored).
func ACFromEquippedItems(items []Item, dexScore int) (int, string) {
	baseAC := 10
	armorName := ""
	hasShield := false

	for _, item := range items {
		if item.Location != "equipped" {
			continue
		}
		lower := strings.ToLower(item.Name)
		if lower == "shield" {
			hasShield = true
			continue
		}
		if armor, ok := armors[lower]; ok {
			if armor.AC > baseAC {
				baseAC = armor.AC
				armorName = item.Name
			}
		}
	}

	ac := baseAC + Modifier(dexScore)
	if hasShield {
		ac++
	}
	return ac, armorName
}

// EquippedWeapons returns stats for all equipped weapons.
func EquippedWeapons(items []Item) []EquippedWeapon {
	var result []EquippedWeapon
	for _, item := range items {
		if item.Location != "equipped" {
			continue
		}
		if w, ok := weapons[strings.ToLower(item.Name)]; ok {
			result = append(result, EquippedWeapon{
				Name:      item.Name,
				Damage:    w.Damage,
				Qualities: w.Qualities,
			})
		}
	}
	return result
}

// ArmorStats returns stats for armor by name (case-insensitive).
func ArmorStats(name string) (Armor, bool) {
	a, ok := armors[strings.ToLower(name)]
	return a, ok
}

// ItemWeight returns the weight in coins for a known item name (case-insensitive).
// Returns the weight and true if found, or 0 and false if unknown.
func ItemWeight(name string) (int, bool) {
	lower := strings.ToLower(name)
	// Check weapons first
	if w, ok := weapons[lower]; ok {
		return w.Weight, true
	}
	// Then armor
	if a, ok := armors[lower]; ok {
		return a.Weight, true
	}
	// Then general items
	if w, ok := itemWeights[lower]; ok {
		return w, true
	}
	return 0, false
}
