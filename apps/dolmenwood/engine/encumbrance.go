package engine

// Item represents an inventory item for encumbrance calculations.
type Item struct {
	ID             uint
	Name           string
	Quantity       int
	Location       string // "equipped", "stowed", or "horse" (legacy, used for backward compat)
	WeightOverride *int   // if set, use this instead of catalog lookup (coins per unit)
	ContainerID    *uint  // parent container item ID (nil = not in a container)
	CompanionID    *uint  // companion ID (nil = not on a companion)
}

// IsEquippedOnCharacter returns true if the item is directly equipped on the character
// (not inside a container and not on a companion).
func (item Item) IsEquippedOnCharacter() bool {
	return item.ContainerID == nil && item.CompanionID == nil
}

// FindRoot walks up the ContainerID chain to find the root item.
func FindRoot(item Item, itemsByID map[uint]Item) Item {
	for item.ContainerID != nil {
		parent, ok := itemsByID[*item.ContainerID]
		if !ok {
			break
		}
		item = parent
	}
	return item
}

// CalculateEncumbrance computes slot usage from items with container hierarchy.
// Returns equipped slots, stowed slots (items in equipped containers), and per-companion slot maps.
func CalculateEncumbrance(items []Item) (equipped, stowed int, companionSlots map[uint]int) {
	companionSlots = make(map[uint]int)

	// Build lookup
	byID := make(map[uint]Item, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}

	for _, item := range items {
		slots := ItemSlots(item)

		if item.CompanionID != nil {
			companionSlots[*item.CompanionID] += slots
			continue
		}

		if item.ContainerID != nil {
			// Walk up to root to determine where this item lives
			root := FindRoot(item, byID)
			if root.CompanionID != nil {
				companionSlots[*root.CompanionID] += slots
			} else {
				// Root is equipped on character -> this is stowed
				stowed += slots
			}
			continue
		}

		// Equipped on character
		equipped += slots
	}
	return
}

// UnitWeight returns the weight in coins per unit for this item.
// Uses WeightOverride if set, otherwise looks up the item name in the catalog.
// Returns 0 for unknown items (treats them as weightless).
func (item Item) UnitWeight() int {
	if item.WeightOverride != nil {
		return *item.WeightOverride
	}
	if w, ok := ItemWeight(item.Name); ok {
		return w
	}
	return 0
}

// TotalWeight returns the total weight in coins for this item (unit weight * quantity).
func (item Item) TotalWeight() int {
	return item.UnitWeight() * item.Quantity
}

// SpeedFromSlots returns movement speed based on equipped and stowed slot counts.
// The result is the slower of the equipped-based and stowed-based speeds.
//
// Equipped: 0-3 → 40, 4-5 → 30, 6-7 → 20, 8+ → 10
// Stowed:   0-10 → 40, 11-12 → 30, 13-14 → 20, 15+ → 10
func SpeedFromSlots(equippedSlots, stowedSlots int) int {
	eSpeed := speedForEquipped(equippedSlots)
	sSpeed := speedForStowed(stowedSlots)
	if eSpeed < sSpeed {
		return eSpeed
	}
	return sSpeed
}

func speedForEquipped(slots int) int {
	switch {
	case slots <= 3:
		return 40
	case slots <= 5:
		return 30
	case slots <= 7:
		return 20
	default:
		return 10
	}
}

func speedForStowed(slots int) int {
	switch {
	case slots <= 10:
		return 40
	case slots <= 12:
		return 30
	case slots <= 14:
		return 20
	default:
		return 10
	}
}

// CoinSlots returns the number of inventory slots consumed by coins.
// Every 100 coins (or fraction thereof) takes 1 slot. 0 coins = 0 slots.
func CoinSlots(coins int) int {
	if coins <= 0 {
		return 0
	}
	return (coins + 99) / 100
}

// WeightToSlots converts total weight in coins to slot count.
func WeightToSlots(weight int) int {
	return CoinSlots(weight)
}

// ItemSlots returns the number of gear slots occupied by an item row.
//
// Resolution order:
//  1. Equipped containers in use: 0 slots
//  2. WeightOverride set: weight-based (escape hatch for custom items)
//  3. Bundled items: ceil(qty / bundleSize) * slotsPerUnit
//  4. Explicit slot category (armor, weapon, clothing, tiny, bulky): cost * qty
//  5. Known weight in catalog: weight-based (ceil(totalWeight / 100))
//  6. Unknown items: 1 per unit
func ItemSlots(item Item) int {
	// Personal containers (backpack, sack, belt pouch) are 0 when equipped/in-use
	if IsPersonalContainer(item.Name) {
		if item.IsEquippedOnCharacter() || item.Location == "equipped" {
			return 0
		}
	}

	// Items with WeightOverride use weight-based calculation
	if item.WeightOverride != nil {
		return WeightToSlots(item.TotalWeight())
	}

	// Bundled items
	if bundle := itemBundleSize(item.Name); bundle > 0 {
		cost := ItemSlotCost(item.Name)
		return ((item.Quantity + bundle - 1) / bundle) * cost
	}

	// Items with explicit slot category
	if cost, explicit := itemSlotCostExplicit(item.Name); explicit {
		return cost * item.Quantity
	}

	// General items with known weight: weight-based
	if w, ok := ItemWeight(item.Name); ok {
		return WeightToSlots(w * item.Quantity)
	}

	// Unknown items: 1 slot per unit
	return item.Quantity
}

// TotalEquippedSlots returns the total slots used by equipped items.
// Containers worn/carried (in use) are 0 slots.
func TotalEquippedSlots(items []Item) int {
	total := 0
	for _, item := range items {
		if item.Location == "equipped" {
			total += ItemSlots(item)
		}
	}
	return total
}

// TotalStowedSlots returns the total slots used by stowed items.
func TotalStowedSlots(items []Item) int {
	total := 0
	for _, item := range items {
		if item.Location == "stowed" {
			total += ItemSlots(item)
		}
	}
	return total
}

// TotalHorseSlots returns the total slots used by items on a horse.
func TotalHorseSlots(items []Item) int {
	total := 0
	for _, item := range items {
		if item.Location == "horse" {
			total += ItemSlots(item)
		}
	}
	return total
}

// ContainerInfo describes a container providing stowed capacity.
type ContainerInfo struct {
	Name  string
	Slots int
}

const maxStowedSlots = 16

// StowedCapacity calculates the total stowed slot capacity from equipped personal containers.
// Only personal containers (backpack, sack, belt pouch) count toward stowed capacity.
// Returns the total capacity (capped at 16) and a list of contributing containers.
func StowedCapacity(items []Item) (int, []ContainerInfo) {
	var containers []ContainerInfo
	total := 0
	for _, item := range items {
		if item.Location != "equipped" && !item.IsEquippedOnCharacter() {
			continue
		}
		if item.Location != "" && item.Location != "equipped" {
			continue
		}
		if !IsPersonalContainer(item.Name) {
			continue
		}
		if cap, ok := ContainerCapacity(item.Name); ok {
			slots := cap * item.Quantity
			containers = append(containers, ContainerInfo{Name: item.Name, Slots: slots})
			total += slots
		}
	}
	if total > maxStowedSlots {
		total = maxStowedSlots
	}
	return total, containers
}
