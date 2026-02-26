# Companions

## Overview (`engine/companions.go`)

Companions are mounts, pack animals, and retainers that travel with a character. They can carry items, wear gear, and have their own combat stats.

There are two distinct companion systems:
- **Animal companions** (mounts and pack animals) are `Companion` records with breed-derived stats.
- **Townsfolk retainers** are also `Companion` records (breed "Townsfolk") with simple flat stats.
- **Adventurer retainers** are full `Character` records linked via a `RetainerContract`. See [Retainer Contracts](retainer-contracts.md).

## Breeds

Seven breeds are supported for animal companions and townsfolk:

| Breed            | AC | HP Max | Speed | Load | Attack | Morale | Saddle? |
|------------------|----|--------|-------|------|--------|--------|---------|
| Charger          | 13 | 16     | 60    | 30   | 2×hoof 1d6, 1×bite 1d4 | 9 | Yes |
| Dapple-doff      | 13 | 12     | 40    | 30   | 2×hoof 1d4 | 7 | Yes |
| Hop-clopper      | 13 | 10     | 40    | 20   | 2×hoof 1d4 | 7 | Yes |
| Mule             | 13 | 10     | 40    | 20   | 1×bite 1d4 | 8 | Yes |
| Prigwort prancer | 13 | 14     | 60    | 30   | 2×hoof 1d6 | 8 | Yes |
| Yellow-flank     | 13 | 8      | 40    | 20   | 2×hoof 1d4 | 6 | Yes |
| Townsfolk        | 10 | 4      | 40    | 5    | 1×weapon 1d6 | 6 | No |

- **Animal Companion** (custom stats): Breed name for Hunter companions with manually entered stats.

Functions:
- `IsCompanionBreed(name)` -- Case-insensitive check (includes the special "Animal Companion" breed for Hunter companions)
- `IsAnimalCompanionBreed(name)` -- Case-insensitive check for the Hunter companion breed

## Townsfolk Retainers

`IsRetainer(breed)` -- Returns true only for "Townsfolk".

`RetainerLoyalty(chaMod)` -- Initial loyalty = 7 + CHA modifier.

Townsfolk retainers are simple NPCs (torch-bearers, porters). They don't need saddles, don't gain XP, and never advance in level. They are modeled as `Companion` records.

## Adventurer Retainers

Adventurer retainers are independent Level 1+ characters of a specific class. Unlike townsfolk, they:
- Are full `Character` records with their own character sheets
- Gain XP (halved by default, configurable via contract)
- Can advance in level
- Count as party members for XP division
- Can be played as standalone characters

Adventurer retainers are linked to their employer via a `RetainerContract` record. See [Retainer Contracts](retainer-contracts.md) for the full specification.

On the employer's character sheet, adventurer retainers appear with:
- Full inline stat block (AC, HP, attack, saves, speed, morale, loyalty)
- Full inventory (readable and transferable from the employer's sheet)
- Link to their own standalone character sheet

## Companion Gear

Three items are classified as companion gear:
- **Pack Saddle** -- Enables full load capacity
- **Riding Saddle** -- Enables 5 slots (saddlebags)
- **Horse Barding** -- +2 AC bonus

`IsCompanionGear(name)` -- Checks if an item is companion gear. Strips magic bonus prefix. Companion gear is special: it does **not consume companion load slots** (it enables capacity rather than using it).

### Saddle Types

`CompanionSaddleTypeFromItems(items)` -- Scans a companion's items for a saddle:
- "Pack Saddle" → `"pack"`
- "Riding Saddle" → `"riding"`
- No saddle → `""`

`CompanionLoadCapacity(breedCapacity, saddleType)`:
- No saddle: **0** capacity (companion can't carry anything)
- Riding saddle: **5** slots (saddlebags)
- Pack saddle: **full breed capacity**

### Barding

`CompanionHasBardingFromItems(items)` -- Scans for "Horse Barding" in companion items.

`BardingACBonus(name)` -- Returns +2 for "Horse Barding", 0 otherwise.

`CompanionAC(baseAC, hasBarding)` -- Base AC + 2 if barding equipped.

## Companion Inventory

Items can be placed on companions by setting `CompanionID` on the item. Companions can also have containers (e.g., a saddlebag modeled as an item on the companion).

The encumbrance system (`CalculateEncumbrance`) tracks companion slots separately from character slots. Companion gear (saddles, barding) is excluded from slot consumption.

## Companion View Model

`CompanionView` wraps `db.Companion` with engine-derived stats:
- AC computed from breed base + barding (or custom stats for Animal Companion)
- Speed from breed (or custom stats for Animal Companion)
- Load capacity from breed + saddle type (or custom stats for Animal Companion)
- Level, saves, attack, morale from breed data (or custom stats for Animal Companion)
- Loyalty (for retainers)
- Saddle type and barding status derived from the companion's items

## Companion in the DB

The `db.Companion` model has:
- `Name`, `Breed`, `HPCurrent`, `HPMax`, `Loyalty`
- `AC`, `Speed`, `LoadCapacity`, `Level`, `Attack`, `Morale` (custom stats for Animal Companion)
- `HasBarding`, `SaddleType` (legacy fields, now derived from items in views)

Deleting a companion moves all its items to the character (sets `CompanionID = nil`, `ContainerID = nil`).
