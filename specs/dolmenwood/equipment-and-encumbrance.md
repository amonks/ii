# Equipment and Encumbrance

## Item Catalog (`engine/equipment.go`)

### Weapons

24 weapons defined with damage dice, qualities (e.g., "melee", "missile", "two-handed"), and weight in coins. Examples: Battle Axe (1d8, melee/two-handed, 50), Dagger (1d4, melee/missile, 10), Longsword (1d8, melee, 60).

`WeaponStats(name)` -- Case-insensitive lookup. Handles magic bonus prefix ("+2 Longsword" finds "Longsword"). Returns `Weapon{Damage, Qualities, Weight}`.

`EquippedWeapons(items)` -- Returns stats for all equipped weapons. Magic bonuses are incorporated into damage strings ("+2 Longsword" yields damage "1d8+2").

### Armor

8 armor types including shield. Each has AC, weight, and Bulk (slot cost: 1=light, 2=medium, 3=heavy):

| Armor        | AC | Weight | Bulk |
|--------------|-----|--------|------|
| Shield       | +1  | 100    | 1    |
| Leather      | 12  | 150    | 1    |
| Scale Mail   | 13  | 300    | 2    |
| Chainmail    | 14  | 400    | 2    |
| Plate Mail   | 16  | 500    | 3    |
| (etc.)       |     |        |      |

`ArmorStats(name)` -- Case-insensitive with magic bonus stripping.

### General Items

~80 items with weights (in coins): containers, light sources, camping gear, holy items, ammunition, tools, clothing, horse accessories, coins, treasure items.

### Containers

12 containers with slot capacities:
- **Personal** (0 equipped slots, add stowed capacity): Backpack (10), Sack (10), Belt Pouch (1)
- **Heavy**: Chests (various), Scroll Case (1)
- **Vehicles**: Cart (100), Wagon (200)

### Magic Bonus Prefix

`ParseMagicBonus(name)` -- Strips "+N " prefix from item names. "+2 Longsword" returns ("Longsword", 2). Used throughout the catalog lookup functions.

## Encumbrance System (`engine/encumbrance.go`)

### Slot-Based System

Characters have two slot categories:
- **Equipped slots**: Items directly on the character (max reasonable is ~10 before speed penalties)
- **Stowed slots**: Items in personal containers (backpacks, sacks, belt pouches), capped at **16 slots maximum**

### Item Slot Costs

Resolution order for `ItemSlots(item)`:
1. Personal containers equipped on character = 0 slots (they grant stowed capacity instead)
2. Items with `WeightOverride` use weight-to-slots conversion
3. Bundled items: ceiling(quantity / bundleSize) slots (e.g., torches bundle at 3, so 7 torches = 3 slots)
4. Explicit slot costs from catalog:
   - **0 slots**: Clothing, tiny items (gems, rings, holy symbols)
   - **2 slots**: Bulky items (10' pole, barrel, large chests)
   - **Bundled**: Candles/10, Torches/3, Arrows/20, Bolts/30, Sling Stones/20, etc.
5. Armor: uses Bulk value (1-3)
6. Weapons: two-handed melee = 2, all others = 1
7. Personal containers = 0
8. Default: 1 slot per unit

### Container Hierarchy

Items form a tree:
- Items can be inside containers (`ContainerID`)
- Items/containers can be on companions (`CompanionID`)
- `FindRoot(item, itemsByID)` walks up the container chain to find the root-level item

### `CalculateEncumbrance(items)`

The main calculation:
1. For each item, find its root via `FindRoot`
2. If root has no companion:
   - If root is equipped on character: add to `equipped` slots
   - If root is a personal container: items inside add to `stowed` slots
3. If root has a companion: add to `companionSlots[companionID]`
4. **Companion gear** (saddles, barding) is explicitly excluded from slot consumption

### Stowed Capacity

`StowedCapacity(items)` -- Counts total stowed slots from personal containers equipped on the character:
- Backpack: 10
- Sack: 10
- Belt Pouch: 1
- Capped at 16 total

Returns both the total capacity and a list of `ContainerInfo{Name, Slots}` for display.

### Speed Calculation

`SpeedFromSlots(equippedSlots, stowedSlots)` -- Speed is determined by the **slower** of two tiers:

**Equipped tier** (based on 10-slot scale):
| Slots | Speed |
|-------|-------|
| 0-3   | 40    |
| 4-6   | 30    |
| 7-9   | 20    |
| 10+   | 10    |

**Stowed tier** (based on 16-slot scale):
| Slots | Speed |
|-------|-------|
| 0-4   | 40    |
| 5-9   | 30    |
| 10-13 | 20    |
| 14+   | 10    |

### Coin Weight

`CoinSlots(coins)` -- Every 100 coins (or fraction thereof) occupies 1 slot. `WeightToSlots(weight)` is an alias.

### Speed Variants

The view model computes multiple speed types from the base speed:
- **Encounter speed**: base (e.g., 40')
- **Exploration (unknown)**: base * 3 (e.g., 120')
- **Exploration (mapped)**: base * 3 * 1.5 (e.g., 180'), ceiled and rounded to nearest 10
- **Running**: base * 3 (e.g., 120')
- **Overland**: base / 5 (e.g., 8 miles/day)

## Data Types

- `Item{ID, Name, Quantity, Location, WeightOverride, ContainerID, CompanionID, IsTiny}` -- Core inventory item
- `ContainerInfo{Name, Slots}` -- Container display info
- `Weapon{Damage, Qualities, Weight}`
- `Armor{AC, Weight, Bulk}`
- `Container{Slots, Personal}`
- `EquippedWeapon{Name, Damage, Qualities, MagicBonus}`
- `SlotCostInfo{SlotsPerUnit, BundleSize}`
