package engine

// Item represents an inventory item for encumbrance calculations.
type Item struct {
	Name           string
	Quantity       int
	Location       string // "equipped", "stowed", or "horse"
	WeightOverride *int   // if set, use this instead of catalog lookup (coins per unit)
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
// Equipped containers (in use) are 0 slots.
func ItemSlots(item Item) int {
	if item.Location == "equipped" && IsContainer(item.Name) {
		return 0
	}
	return WeightToSlots(item.TotalWeight())
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

// StowedCapacity calculates the total stowed slot capacity from equipped containers.
// Returns the total capacity (capped at 16) and a list of contributing containers.
func StowedCapacity(items []Item) (int, []ContainerInfo) {
	var containers []ContainerInfo
	total := 0
	for _, item := range items {
		if item.Location != "equipped" {
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
