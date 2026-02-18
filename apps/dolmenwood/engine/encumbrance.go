package engine

// Item represents an inventory item for encumbrance calculations.
type Item struct {
	SlotCost int
	Quantity int
	Location string // "equipped", "stowed", or "companion:{id}"
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

// TotalEquippedSlots returns the total slots used by equipped items.
func TotalEquippedSlots(items []Item) int {
	total := 0
	for _, item := range items {
		if item.Location == "equipped" {
			total += item.SlotCost * item.Quantity
		}
	}
	return total
}

// TotalStowedSlots returns the total slots used by stowed items.
func TotalStowedSlots(items []Item) int {
	total := 0
	for _, item := range items {
		if item.Location == "stowed" {
			total += item.SlotCost * item.Quantity
		}
	}
	return total
}
