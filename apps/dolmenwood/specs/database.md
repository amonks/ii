# Database Layer

## Overview

The `db` package provides SQLite persistence via GORM. It defines all data models, handles schema creation and migrations, and exposes CRUD methods. Located in `db/db.go`.

## Schema (9 tables)

### `characters`

The core entity. Fields:
- **Identity**: `ID`, `Name`, `Class`, `Kindred`, `Level`
- Defaults: `class` (text, no default), `kindred` (text, no default)
- **Ability Scores**: `STR`, `DEX`, `CON`, `INT`, `WIS`, `CHA`
- **Combat**: `HPCurrent`, `HPMax`
- **Background**: `Alignment`, `Background`, `Liege`
- **Found Treasure**: `FoundCP`, `FoundSP`, `FoundEP`, `FoundGP`, `FoundPP` -- coins found but not yet "returned to safety"
- **Purse (legacy)**: `PurseCP`, `PurseSP`, `PurseEP`, `PurseGP`, `PursePP` -- now computed from inventory, these fields are vestigial
- **Coin Location**: `CoinCompanionID`, `CoinContainerID` -- where the character's consolidated coin item lives
- **XP**: `TotalXP`
- **Calendar**: `CurrentDay` (game day counter), `CalendarStartDay` (maps game days to calendar dates)
- **Birthday**: `BirthdayMonth`, `BirthdayDay` (for moon sign computation)
- **Timestamps**: `CreatedAt`, `UpdatedAt`

### `items`

Inventory items belonging to a character. Fields:
- `ID`, `CharacterID`, `Name`, `Quantity`, `Location` (legacy: "equipped"/"stowed"/"horse")
- `WeightOverride` (nullable, overrides catalog weight)
- `Notes` (freeform, used for coin denomination details like "50gp 20sp")
- `SortOrder`
- `ContainerID` (nullable FK to items -- items can nest inside containers)
- `CompanionID` (nullable FK to companions -- items can be on companions)
- `IsTiny` (boolean, affects slot calculation)

### `companions`

Animal companions, mounts, and townsfolk retainers. Fields:
- `ID`, `CharacterID`, `Name`, `Breed`
- `HPCurrent`, `HPMax`
- `HasBarding`, `SaddleType` (legacy, now derived from items)
- `Loyalty` (for retainers)

### `retainer_contracts`

Links an employer character to an adventurer retainer character. Fields:
- `ID`
- `EmployerID` (FK to characters) -- the hiring character
- `RetainerID` (FK to characters) -- the retained adventurer character
- `LootSharePct` (float64) -- retainer's share of loot as a percentage (e.g. 15.0 for 15%)
- `XPSharePct` (float64) -- percentage of XP the retainer receives (default 50.0 per rules)
- `DailyWageCP` (int) -- daily wage in copper pieces
- `HiredOnDay` (int) -- game day when the contract started
- `Active` (bool) -- whether the contract is currently active
- `CreatedAt`

Indexed on `employer_id` and `retainer_id`.

CRUD methods:
- `CreateRetainerContract(rc)` -- creates a new contract
- `ListActiveRetainerContracts(employerID)` -- returns active contracts for an employer
- `GetRetainerContract(id)` -- fetches a single contract
- `UpdateRetainerContract(rc)` -- updates contract terms
- `DeactivateRetainerContract(id)` -- sets `active = false`

### `transactions`

Wealth transaction log. Fields:
- `ID`, `CharacterID`, `Amount`, `CoinType`, `Description`
- `IsFoundTreasure` (boolean)
- `CreatedAt`

### `xp_log`

XP gain history. Custom table name `xp_log`. Fields:
- `ID`, `CharacterID`, `Amount`, `Description`, `CreatedAt`

### `notes`

Freeform character notes. Fields:
- `ID`, `CharacterID`, `Content`, `CreatedAt`

### `audit_log`

Activity log tracking all character changes. Fields:
- `ID`, `CharacterID`, `Action`, `Detail`, `GameDay`, `CreatedAt`

### `bank_deposits`

In-game bank deposits. Fields:
- `ID`, `CharacterID`, `CoinNotes` (denomination string like "50gp"), `CPValue` (total value in copper), `DepositDay` (game day of deposit)

## Cascade Behaviors

### `DeleteCharacter(id)`
Deletes all related records: items, companions, transactions, XP log, notes, audit log, bank deposits. Also deletes any `retainer_contracts` where `employer_id` or `retainer_id` matches. When deleting an employer, retainer Characters remain as independent characters (just the contract is removed).

### `DeleteItem(id)`
- Reparents child items: any item whose `ContainerID` points to the deleted item gets `ContainerID = nil`
- Reparents coin location: if a character's `CoinContainerID` or `CoinCompanionID` references the deleted item's container/companion context, those refs are cleared

### `DeleteCompanion(id)`
- Moves all companion items to the character: sets `CompanionID = nil` and `ContainerID = nil` on all items belonging to that companion
- Clears character's `CoinCompanionID` if it pointed to this companion

## Item Transfers

### `TransferItem(itemID, toCharacterID, quantity)`

Moves an item (or partial stack) from one character to another:
- Full transfer: updates `item.CharacterID` to the target, clears `ContainerID` and `CompanionID`
- Partial transfer (quantity < item.Quantity): creates a new item on the target character with the specified quantity, reduces the source item's quantity
- Clears the source character's coin location when the moved item is the consolidated coin item or the container holding it
- When transferring a container item, all child items (items with `ContainerID` pointing at the container) are updated to the target character as well

## Key Business Logic

### `ReturnToSafety(characterID, xpModPercent)`

The one piece of business logic in the DB layer:
1. Calculates found treasure GP value: `FoundGP + FoundSP/10 + FoundCP/100 + FoundPP*5 + FoundEP/2`
2. Applies XP modifier: `gpValue + (gpValue * modPercent / 100)`
3. Adds resulting XP to `TotalXP`
4. Zeroes all `Found*` fields
5. Creates XP log entry
6. Creates audit log entry recording the XP gain

## Migrations

Schema creation uses GORM's `AutoMigrate` for the model types. Additional migrations run as best-effort `ALTER TABLE` statements that silently fail if already applied. This includes:
- Adding columns like `CoinCompanionID`, `CoinContainerID`, `IsTiny`, `Loyalty`
- Adding `BirthdayMonth`, `BirthdayDay` columns
- Adding the `bank_deposits` table
- Adding the `retainer_contracts` table
- Running `migrateEPtoSP()` to convert electrum pieces to silver (EP doesn't exist in Dolmenwood)

## Test Infrastructure

`NewMemory()` creates an in-memory SQLite database for testing. Tests cover CRUD round-trips, cascade deletes, item hierarchy (containers, companions), audit logging, the ReturnToSafety XP calculation, and retainer contract operations.
