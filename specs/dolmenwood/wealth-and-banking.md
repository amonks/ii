# Wealth and Banking

## Coin System (`engine/wealth.go`)

### Denominations

Five coin types in order of value:
- **CP** (Copper Pieces) -- base unit
- **SP** (Silver Pieces) -- 10 CP
- **EP** (Electrum Pieces) -- 50 CP (legacy, migrated to SP)
- **GP** (Gold Pieces) -- 100 CP
- **PP** (Platinum Pieces) -- 500 CP

Display order: PP, GP, EP, SP, CP (highest to lowest).

Note: Electrum doesn't exist in Dolmenwood. The database runs a migration (`migrateEPtoSP`) to convert any EP to SP.

### Coin Purse

`CoinPurse{CP, SP, EP, GP, PP}` -- holds counts of each denomination.

- `CoinPurseGPValue(purse)` -- Converts to gold piece value
- `TotalCoins(purse)` -- Sum of all individual coins
- `AddToPurse(purse, amount, coinType)` -- Returns new purse with addition
- `MinCoins(cpValue)` -- Converts CP to fewest coins (GP/SP/CP only, no PP/EP)

### Inventory Coins

Coins in the character's inventory are stored as a single `Item` named "Coins" with a `Notes` field containing the denomination breakdown (e.g., "50gp 20sp 10cp").

- `CoinItemNameStr = "Coins"` -- The consolidated coin item name
- `IsCoinItem(name)` -- Recognizes both "Coins" and legacy per-denomination names ("Gold Pieces", etc.). Strips magic bonus prefix.
- `CoinItemName(coinType)` / `CoinTypeFromItemName(name)` -- Bidirectional lookups for legacy names

### Parsing and Formatting

#### Transaction Parsing
`ParseTransaction(input)` -- Parses shorthand like "50g dragon hoard" or "-10sp ale":
- Amount (positive or negative integer)
- Coin type shorthand: `g`/`gp`=GP, `s`/`sp`=SP, `c`/`cp`=CP, `p`/`pp`=PP, `e`/`ep`=EP
- Remaining text = description
- Returns `(amount, coinType, description, error)`

#### Coin Expression Parsing
`ParseCoinExpression(input)` -- Parses free-text like "100gp 2sp" into `[]CoinAmount`. Validates that the entire input is consumed (rejects trailing garbage).

#### Coin Notes
- `FormatCoinNotes(coins map)` -- Map to string: "50gp 20sp 10cp"
- `ParseCoinNotes(notes)` -- String to map
- `MergeCoinNotes(existing, add)` -- Add coins to existing notes string, returns new string and total CP
- `SubtractCoinNotes(existing, sub)` -- Subtract coins with insufficient-funds error checking
- `CoinNotesGPValue(notes)` -- Parse and compute GP value

### Wealth in the View Model

The view computes several wealth displays:
- **InventoryCoins**: All coin items across all locations (character + companions)
- **PurseCoins**: InventoryCoins minus FoundTreasure (what the character "safely has")
- **FoundGPValue/FoundLabel**: Found treasure awaiting return to safety
- **TotalCoinsLabel**: Simplified coin display (e.g., "52gp 1sp" instead of "50gp 20sp 10cp")

Simplification converts smaller denominations upward: 10sp becomes 1gp, 100cp becomes 1gp.

## Found Treasure System

Characters track found treasure separately from their purse. Found treasure represents coins discovered during adventuring but not yet secured.

### Flow
1. **Add Treasure** (`handleAddTreasure`): Parses input, creates transaction, updates Found* fields on character, creates/merges coin inventory item
2. **Return to Safety** (`handleReturnToSafety` / `db.ReturnToSafety`): Converts found treasure GP value to XP (with modifier), zeroes Found* fields, logs XP gain
3. **Undo Transaction** (`handleUndoTransaction`): Creates inverse transaction, reverses found treasure accounting

Found treasure tracking uses the character's `FoundCP/SP/EP/GP/PP` fields.

## Banking System (`engine/bank.go`)

### Bank Deposits

Players can deposit coins at an in-game bank. Deposits are tracked as `BankLot{ID, CPValue, DepositDay}`.

### Deposit Maturity

`IsMature(depositDay, currentDay)` -- A deposit matures after **30 game days**. Mature deposits can be withdrawn without penalty.

### Withdrawal Planning

`PlanWithdrawal(lots, requestedCP, currentDay)` -- The core banking algorithm:

1. **Mature lots** (30+ days old) are consumed first, sorted oldest-first, at 1:1 value (no fee)
2. **Immature lots** (less than 30 days) are consumed next, sorted newest-first, with a **10% fee**: the gross amount consumed is larger than the net received
3. For partially consumed lots, the lot's value is reduced rather than deleted
4. Returns a `WithdrawResult{ConsumedLots, UpdatedLots, GrossCP, FeeCP, NetCP}`
5. Returns an error if total available funds are insufficient

### Coin Notes in Deposits

`CoinNotesCPValue(notes)` -- Parses a coin notes string from a deposit and returns its total CP value.
`CoinPurseCPValue(purse)` -- Returns total CP from a CoinPurse (GP/SP/CP only).

### Bank View

The view model wraps deposits with:
- `BankDepositView{IsMature, DaysUntilMature, GPValue}` -- display-friendly deposit info
- `BankTotalCP` -- sum of all deposit values

### Banking Handlers

- **Deposit** (`handleUpdateItem` with `move_to=bank`): Converts a coin item to a bank deposit
- **Withdraw** (`handleBankWithdraw`): Parses coin expression, calls `PlanWithdrawal`, deletes/updates deposit lots, adds coins to inventory
