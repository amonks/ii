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
	Bulk   int // slot cost: 1=light, 2=medium, 3=heavy
}

var weapons = map[string]Weapon{
	"battle axe":       {Damage: "1d8", Qualities: "Melee", Weight: 100},
	"club":             {Damage: "1d4", Qualities: "Melee", Weight: 20},
	"crossbow":         {Damage: "1d8", Qualities: "Armour piercing, Missile (80′ / 160′ / 240′), Reload, Two-handed", Weight: 50},
	"crossbow (ranged)": {Damage: "1d8", Qualities: "Armour piercing, Missile (80′ / 160′ / 240′), Reload, Two-handed", Weight: 50},
	"dagger":           {Damage: "1d4", Qualities: "Melee, Missile", Weight: 10},
	"hand axe":         {Damage: "1d6", Qualities: "Melee, Missile", Weight: 20},
	"holy water vial":  {Damage: "1d8", Qualities: "Missile (10′ / 30′ / 50′), Splash", Weight: 10},
	"lance":            {Damage: "1d6", Qualities: "Brace, Charge, Melee, Reach", Weight: 100},
	"longbow":          {Damage: "1d6", Qualities: "Missile (70′ / 140′ / 210′), Two-handed", Weight: 40},
	"longbow (ranged)": {Damage: "1d6", Qualities: "Missile (70′ / 140′ / 210′), Two-handed", Weight: 40},
	"longsword":        {Damage: "1d8", Qualities: "Melee", Weight: 30},
	"mace":             {Damage: "1d6", Qualities: "Melee", Weight: 40},
	"oil flask (burning)": {Damage: "1d8", Qualities: "Missile (10′ / 30′ / 50′), Splash", Weight: 10},
	"polearm":          {Damage: "1d10", Qualities: "Brace, Melee, Reach, Two-handed", Weight: 140},
	"shortbow":         {Damage: "1d6", Qualities: "Missile (50′ / 100′ / 150′), Two-handed", Weight: 20},
	"shortbow (ranged)": {Damage: "1d6", Qualities: "Missile (50′ / 100′ / 150′), Two-handed", Weight: 20},
	"shortsword":       {Damage: "1d6", Qualities: "Melee", Weight: 20},
	"sling":            {Damage: "1d4", Qualities: "Missile (40′ / 80′ / 160′)", Weight: 10},
	"sling (ranged)":   {Damage: "1d4", Qualities: "Missile (40′ / 80′ / 160′)", Weight: 10},
	"spear":            {Damage: "1d6", Qualities: "Brace, Melee, Missile (20′ / 40′ / 60′)", Weight: 30},
	"spear (ranged)":   {Damage: "1d6", Qualities: "Brace, Melee, Missile (20′ / 40′ / 60′)", Weight: 30},
	"staff":            {Damage: "1d4", Qualities: "Melee, Two-handed", Weight: 40},
	"torch (flaming)":     {Damage: "1d4", Qualities: "Melee", Weight: 10},
	"two-handed sword":    {Damage: "1d10", Qualities: "Melee, Two-handed", Weight: 140},
	"war hammer":       {Damage: "1d6", Qualities: "Melee", Weight: 40},
}

var armors = map[string]Armor{
	"leather":    {AC: 12, Weight: 200, Bulk: 1},
	"bark":       {AC: 13, Weight: 300, Bulk: 1},
	"chainmail":  {AC: 14, Weight: 400, Bulk: 2},
	"chain mail": {AC: 14, Weight: 400, Bulk: 2},
	"pinecone":   {AC: 15, Weight: 400, Bulk: 2},
	"plate mail": {AC: 16, Weight: 500, Bulk: 3},
	"full plate": {AC: 17, Weight: 700, Bulk: 3},
	"shield":     {AC: 1, Weight: 100, Bulk: 1},
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
	"hammer (small)":        30,
	"hammer (sledgehammer)": 100,
	"sledgehammer":          100,
	"ink":               5,
	"iron spikes":       5,  // per spike
	"lock":              10,
	"magnifying glass":  5,
	"manacles":          60,
	"marbles":           1,  // per marble
	"mining pick":       100,
	"mirror":            50,
	"mirror (small)":    50,
	"musical instrument":            50,
	"musical instrument (stringed)": 50,
	"musical instrument (wind)":     20,
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
	"clothes":              30,
	"clothes, common":      30,
	"clothes, extravagant": 60,
	"clothes, fine":        40,
	"habit, friar's":       30,
	"robes":                30,
	"robes, ritual":        30,
	"winter cloak":         20,

	// Tiny items
	"pipeleaf": 0,

	// Horse accessories
	"feed":                     20,
	"horse barding":            600,
	"canoe":                    500,
	"pack saddle and bridle":   150,
	"riding saddle and bridle": 300,
	"riding saddle bags":       100,

	// Coins (1 coin = 1 cn weight)
	"coins":           1,
	"copper pieces":   1,
	"silver pieces":   1,
	"electrum pieces": 1,
	"gold pieces":     1,
	"platinum pieces": 1,

	// Treasure items (weight per unit)
	"gem":       1,
	"jewellery": 10,
	"potion":    10,
	"rod":       20,
	"scroll":    1,
	"wand":      10,
}

// Container holds the slot capacity and type for a known container.
type Container struct {
	Slots    int  // item capacity in slots
	Personal bool // personal containers (backpack, sack, belt pouch) are 0 slots when equipped
}

var containers = map[string]Container{
	// Personal containers: 0 slots when equipped, provide stowed capacity
	"backpack":   {Slots: 10, Personal: true},
	"sack":       {Slots: 10, Personal: true},
	"belt pouch": {Slots: 1, Personal: true},

	// Heavy containers: always cost their bulky/general slot cost, but can hold items
	"casket (iron, large)":   {Slots: 8},  // 800 coins / 100
	"casket (iron, small)":   {Slots: 3},  // ceil(250 / 100)
	"chest (wooden, large)":  {Slots: 10}, // 1000 coins / 100
	"chest (wooden, small)":  {Slots: 3},  // ceil(300 / 100)
	"scroll case":            {Slots: 1},  // 1 scroll
	"riding saddle bags":     {Slots: 5},  // 500 coins / 100

	// Vehicles
	"cart":  {Slots: 100}, // 10,000 coins / 100
	"wagon": {Slots: 200}, // 20,000 coins / 100
}

// ContainerCapacity returns the slot capacity for a known container (case-insensitive).
func ContainerCapacity(name string) (int, bool) {
	c, ok := containers[strings.ToLower(name)]
	return c.Slots, ok
}

// IsContainer returns whether the item name is a known container.
func IsContainer(name string) bool {
	_, ok := containers[strings.ToLower(name)]
	return ok
}

// IsPersonalContainer returns whether the item is a personal container
// (backpack, sack, belt pouch) that is 0 slots when equipped.
func IsPersonalContainer(name string) bool {
	c, ok := containers[strings.ToLower(name)]
	return ok && c.Personal
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
// Deprecated: use CharacterAC to include kindred bonuses.
func ACFromEquippedItems(items []Item, dexScore int) (int, string) {
	return CharacterAC("", items, dexScore)
}

// CharacterAC computes armor class from equipped items, DEX score, and kindred traits.
func CharacterAC(kindred string, items []Item, dexScore int) (int, string) {
	baseAC := 10
	armorName := ""
	armorName, hasShield := ArmorContributors(items)

	if armorName != "" {
		if armor, ok := ArmorStats(armorName); ok {
			baseAC = armor.AC
		}
	}

	bonus := 0
	if strings.EqualFold(kindred, "breggle") {
		if armorName == "" {
			bonus = 1
		} else if armor, ok := ArmorStats(armorName); ok && armor.Bulk == 1 {
			bonus = 1
		}
	}

	ac := baseAC + Modifier(dexScore)
	if hasShield {
		ac++
	}
	ac += bonus
	return ac, armorName
}

// ArmorContributors returns the equipped armor name and whether a shield is equipped.
func ArmorContributors(items []Item) (string, bool) {
	armorName := ""
	hasShield := false
	baseAC := 10

	for _, item := range items {
		if item.Location != "equipped" && !item.IsEquippedOnCharacter() {
			continue
		}
		if item.Location != "" && item.Location != "equipped" {
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

	return armorName, hasShield
}

// EquippedWeapons returns stats for all equipped weapons.
func EquippedWeapons(items []Item) []EquippedWeapon {
	var result []EquippedWeapon
	for _, item := range items {
		if item.Location != "equipped" && !item.IsEquippedOnCharacter() {
			continue
		}
		if item.Location != "" && item.Location != "equipped" {
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

// SlotCostInfo describes the slot cost for a category of items.
type SlotCostInfo struct {
	SlotsPerUnit int // 0, 1, 2, or 3
	BundleSize   int // if >0, SlotsPerUnit applies per this many items
}

// itemSlotCosts maps item names to their slot cost info.
// Items not in this map resolve via armor/weapon lookup or default to 1.
var itemSlotCosts = map[string]SlotCostInfo{
	// 0 slots: clothing
	"clothes":              {SlotsPerUnit: 0},
	"clothes, common":      {SlotsPerUnit: 0},
	"clothes, extravagant": {SlotsPerUnit: 0},
	"clothes, fine":        {SlotsPerUnit: 0},
	"habit, friar's":       {SlotsPerUnit: 0},
	"robes":                {SlotsPerUnit: 0},
	"robes, ritual":        {SlotsPerUnit: 0},
	"winter cloak":         {SlotsPerUnit: 0},

	// 0 slots: tiny items
	"bell":                  {SlotsPerUnit: 0},
	"fungi":                 {SlotsPerUnit: 0},
	"herbs":                 {SlotsPerUnit: 0},
	"holy symbol":           {SlotsPerUnit: 0},
	"holy symbol (wooden)":  {SlotsPerUnit: 0},
	"holy symbol (silver)":  {SlotsPerUnit: 0},
	"holy symbol (gold)":    {SlotsPerUnit: 0},
	"paper":                 {SlotsPerUnit: 0},
	"parchment":             {SlotsPerUnit: 0},
	"pipeleaf":              {SlotsPerUnit: 0},
	"quill":                 {SlotsPerUnit: 0},
	"whistle":               {SlotsPerUnit: 0},

	// 1 slot per bundle
	"candles":      {SlotsPerUnit: 1, BundleSize: 10},
	"torches":      {SlotsPerUnit: 1, BundleSize: 3},
	"torch":        {SlotsPerUnit: 1, BundleSize: 3},
	"caltrops":     {SlotsPerUnit: 1, BundleSize: 20},
	"chalk":        {SlotsPerUnit: 1, BundleSize: 10},
	"iron spikes":  {SlotsPerUnit: 1, BundleSize: 12},
	"marbles":      {SlotsPerUnit: 1, BundleSize: 20},
	"arrows":       {SlotsPerUnit: 1, BundleSize: 20},
	"quarrels":     {SlotsPerUnit: 1, BundleSize: 20},
	"sling stones": {SlotsPerUnit: 1, BundleSize: 20},

	// 2 slots: bulky items
	"barrel":                {SlotsPerUnit: 2},
	"casket (iron, large)":  {SlotsPerUnit: 2},
	"casket (iron, small)":  {SlotsPerUnit: 2},
	"chest (wooden, large)": {SlotsPerUnit: 2},
	"chest (wooden, small)": {SlotsPerUnit: 2},
	"pole":                  {SlotsPerUnit: 2},
	"sledgehammer":          {SlotsPerUnit: 2},
	"hammer (sledgehammer)": {SlotsPerUnit: 2},
	"rope ladder":           {SlotsPerUnit: 2},
	"firewood":              {SlotsPerUnit: 2},
}

var tinyItems = map[string]struct{}{
	"bell":                 {},
	"fungi":                {},
	"herbs":                {},
	"holy symbol":          {},
	"holy symbol (wooden)": {},
	"holy symbol (silver)": {},
	"holy symbol (gold)":   {},
	"paper":                {},
	"parchment":            {},
	"pipeleaf":             {},
	"quill":                {},
	"whistle":              {},
}

// IsTinyItem returns whether the item is a known tiny item.
func IsTinyItem(name string) bool {
	_, ok := tinyItems[strings.ToLower(name)]
	return ok
}

// ItemSlotCost returns the number of gear slots for one unit of the named item.
// Checks: explicit slot cost map -> armor -> weapon -> container -> default 1.
func ItemSlotCost(name string) int {
	cost, _ := itemSlotCostExplicit(name)
	return cost
}

// ItemSlotCostExplicit returns the slot cost and whether it came from an explicit
// category (clothing, tiny, bulky, bundled, armor, weapon, container).
// Items with only a known weight return (1, false) so the caller can fall back
// to weight-based calculation.
func ItemSlotCostExplicit(name string) (int, bool) {
	return itemSlotCostExplicit(name)
}

// itemSlotCostExplicit returns the slot cost and whether it came from an explicit
// category (clothing, tiny, bulky, bundled, armor, weapon, container).
// Items with only a known weight return (1, false) so the caller can fall back
// to weight-based calculation.
func itemSlotCostExplicit(name string) (int, bool) {
	lower := strings.ToLower(name)

	// Explicit slot cost catalog (clothing, tiny, bundled, bulky)
	if info, ok := itemSlotCosts[lower]; ok {
		return info.SlotsPerUnit, true
	}

	// Armor
	if a, ok := armors[lower]; ok {
		return a.Bulk, true
	}

	// Weapon: melee + two-handed = 2 slots, otherwise 1
	if w, ok := weapons[lower]; ok {
		q := strings.ToLower(w.Qualities)
		if strings.Contains(q, "melee") && strings.Contains(q, "two-handed") {
			return 2, true
		}
		return 1, true
	}

	// Personal container (backpack, sack, belt pouch)
	if c, ok := containers[lower]; ok && c.Personal {
		return 1, true
	}

	// Default: general item = 1 slot, but not explicit
	return 1, false
}

// ItemBundleSize returns the bundle size for a named item, or 0 if not bundled.
func ItemBundleSize(name string) int {
	return itemBundleSize(name)
}

// itemBundleSize returns the bundle size for a named item, or 0 if not bundled.
func itemBundleSize(name string) int {
	lower := strings.ToLower(name)
	if info, ok := itemSlotCosts[lower]; ok {
		return info.BundleSize
	}
	return 0
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
