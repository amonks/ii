# Store (Shop System)

## Overview (`server/store.go`)

The store lets characters buy equipment from a hardcoded catalog and sell items from their inventory. All transactions use the character's coin inventory.

## Store Catalog

Defined in `buildStoreGroups()` in `views.go`. Five groups:

1. **Adventuring Gear** (56 items) -- Backpack, rope, torches, rations, etc.
2. **Weapons** (18 items) -- Swords, axes, bows, etc.
3. **Ammunition** (3 items) -- Arrows, bolts, sling stones
4. **Armour** (7 items) -- Leather through plate mail, shield
5. **Horses and Vehicles** (17 items) -- All companion breeds, saddles, carts, wagons

Each store item includes: name, cost in CP, and engine-derived enrichments (weight, bulk/slots, damage, AC, qualities, load capacity, cargo capacity, container capacity).

## Buying

### `handleStoreBuy()`

1. Parses store item ID (format: `name|costCP`)
2. Three purchase paths:
   - **Free items** (cost 0): directly creates item, no coin deduction
   - **Companion breeds**: creates a `db.Companion` instead of an item
   - **Regular items**: deducts coins, creates inventory item

### Coin Deduction (`deductCoins`)

The core spending logic:

1. Finds spendable coin items via `findSpendableCoins()` -- prefers coins directly on the character (not in containers or on companions)
2. Calculates the character's spendable purse (inventory coins minus found treasure)
3. Checks if purse has sufficient funds (total CP value)
4. Performs changemaking:
   - Converts entire purse to CP
   - Subtracts the cost
   - Converts remainder back to fewest coins via `minCoinCounts()`
   - **Preserves PP** (platinum pieces are not spent)
   - Uses only GP, SP, CP for change

### Changemaking (`minCoinCounts`)

Converts a CP value to the fewest GP/SP/CP coins:
- GP = cpValue / 100
- SP = remainder / 10
- CP = remainder

EP (electrum) is excluded (doesn't exist in Dolmenwood). PP (platinum) is excluded (treated as treasure, not currency).

## Selling

### `handleSellItem()`

Sells an entire item stack at **half the store price** (rounded down).

### `sellItemQuantity()`

Core selling logic:
1. Looks up the store sell price via `storeSellPriceCP(name)` -- half of the buy price
2. Removes the item from inventory
3. Adds the coin value to the character's coin inventory
4. Creates audit log entry

### Sell Price

`storeSellPriceCP(name)` -- Scans the store catalog for the item name, returns half the buy price. Items not in the store catalog cannot be sold (returns 0).

## Finding Spendable Coins

`findSpendableCoins(items)` -- Returns coin items sorted by preference:
1. Coins directly on the character (no container, no companion) -- preferred
2. All other coin items

This ensures the character spends coins from their person first, rather than from saddlebags or containers.

## Coin Display

`cpAsCoinLabel(cp)` -- Converts a CP value to a human-readable label like "5gp" or "2sp 5cp". Used for displaying prices and sell values.

## Item ID Format

Store items are identified by a composite key: `name|costCP` (e.g., `"Longsword|1000"`). The `storeItemSeparator` constant is `"|"`. This format is used in form values to identify which store item to buy.
