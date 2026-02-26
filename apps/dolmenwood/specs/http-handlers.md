# HTTP Handlers

## Route Registration (`server/server.go`)

All routes are registered via `Mux()` on the `Server` struct, using Go 1.22+ method-based routing.

## Routes

### Character Management
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/{$}` | `handleIndex` | Character list page |
| POST | `/characters/` | `handleCreateCharacter` | Create new character (with class/kindred selection) |
| GET | `/characters/{id}/` | `handleCharacterSheet` | Full character sheet |
| POST | `/characters/{id}/delete/` | `handleDeleteCharacter` | Delete character (cascade) |

### Combat Stats
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/characters/{id}/hp/` | `handleUpdateHP` | Update current HP |
| POST | `/characters/{id}/birthday/` | `handleUpdateBirthday` | Set birthday (for moon sign) |

### Inventory
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/characters/{id}/items/` | `handleAddItem` | Add item to inventory |
| POST | `/characters/{id}/items/{itemID}/update/` | `handleUpdateItem` | Move/edit/bank-deposit item |
| POST | `/characters/{id}/items/{itemID}/split/` | `handleSplitItem` | Split stack or partial coins |
| POST | `/characters/{id}/items/{itemID}/decrement/` | `handleDecrementItem` | Remove one bundle from bundled item |
| POST | `/characters/{id}/items/{itemID}/delete/` | `handleDeleteItem` | Delete item |
| POST | `/characters/{id}/items/{itemID}/sell/` | `handleSellItem` | Sell item at half price |

### Store
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/characters/{id}/store/buy/` | `handleStoreBuy` | Buy item from store |

### Companions
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/characters/{id}/companions/` | `handleAddCompanion` | Add companion |
| POST | `/characters/{id}/companions/{compID}/update/` | `handleUpdateCompanion` | Update companion stats |
| POST | `/characters/{id}/companions/{compID}/delete/` | `handleDeleteCompanion` | Delete companion |

### Adventurer Retainers
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/characters/{id}/retainers/` | `handleHireRetainer` | Create retainer Character + contract |
| POST | `/characters/{id}/retainers/{contractID}/dismiss/` | `handleDismissRetainer` | Deactivate retainer contract |
| POST | `/characters/{id}/retainers/{contractID}/update/` | `handleUpdateRetainerContract` | Update contract terms |
| POST | `/characters/{id}/retainers/{contractID}/transfer/` | `handleTransferItem` | Transfer item to/from retainer |
| POST | `/characters/{id}/retainers/{contractID}/items/` | `handleAddRetainerItem` | Add item directly to retainer inventory |

### Wealth
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/characters/{id}/treasure/` | `handleAddTreasure` | Record found/purse treasure |
| POST | `/characters/{id}/treasure/{txID}/undo/` | `handleUndoTransaction` | Reverse a transaction |
| POST | `/characters/{id}/return-to-safety/` | `handleReturnToSafety` | Convert found treasure to XP |

### XP & Advancement
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/characters/{id}/xp/` | `handleAddXP` | Grant XP directly |
| POST | `/characters/{id}/level-up/` | `handleLevelUp` | Level up character |

### Notes
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/characters/{id}/notes/` | `handleAddNote` | Add note |
| POST | `/characters/{id}/notes/{noteID}/delete/` | `handleDeleteNote` | Delete note |

### Calendar
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/characters/{id}/calendar/` | `handleUpdateCalendar` | Set calendar date |
| POST | `/characters/{id}/advance-day/` | `handleAdvanceDay` | Advance game day counter |

### Banking
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/characters/{id}/bank/withdraw/` | `handleBankWithdraw` | Withdraw from bank |

### Fonts
4 routes serving embedded woff2 font files with immutable cache headers.

## Key Handler Behaviors

### `handleCreateCharacter`
Reads `class` and `kindred` from form values (validated via `engine.IsValidClass`/`engine.IsValidKindred`). Creates a Level 1 character with the selected class and kindred.

### `handleAddItem`
Smart input parsing:
- Recognizes coin expressions ("5gp 10sp") and creates/merges a Coins item
- Recognizes negative quantities ("-2x feed") and deducts existing items
- Recognizes "tiny" prefix ("tiny lock of hair") and sets IsTiny flag
- Parses "Nx name" for quantities ("5x torch")
- Supports `move_to` form field for placing directly into containers/companions
- Combines stackable items at the same location (bundled items merge quantities, coins merge denomination notes)

### `handleUpdateItem`
Supports special `move_to` values:
- Container/companion IDs: moves item to that location
- `"consume"`: deletes the item
- `"bank"`: converts a coin item into a bank deposit
- Also handles quantity and notes updates

### `handleSplitItem`
Complex splitting logic:
- For coins: parses coin expression to split partial denominations from a coin item
- For regular items: splits N units into a new item at a target location
- Split targets: bank deposit, consume, sell, move to location

### `handleDecrementItem`
For bundled items (torches come in bundles of 6): decrements by one bundle-worth. If quantity reaches 0, deletes the item.

### `handleAddTreasure`
Parses shorthand ("50g dragon hoard"), creates transaction record, updates Found* fields on character, creates or merges a Coins inventory item with the denomination added to notes.

### `handleReturnToSafety`
Calculates XP modifier via `engine.TotalXPModifier(ch.Kindred, scores, engine.ClassPrimes(ch.Class))`, delegates to `db.ReturnToSafety()` which converts found treasure GP value to XP.

### `handleBankWithdraw`
Parses coin expression, calls `engine.PlanWithdrawal()` to select deposit lots for consumption (mature first, then immature with 10% fee), applies the plan (delete/update deposits), adds coins to inventory.

### `handleHireRetainer`
Creates a new `Character` with the specified class, kindred, ability scores, and HP. Creates a `RetainerContract` linking the new character to the employer with the specified loot share percentage, XP share percentage, and daily wage. Computes initial loyalty from employer's CHA modifier. Creates audit log entries on both characters.

### `handleDismissRetainer`
Sets the contract's `Active` field to false. The retainer Character remains as an independent character. Creates audit log entries.

## HTMX Partial Update Pattern

Most handlers re-render specific page sections rather than redirecting:
- `renderStats(w, r, charID)` -- re-renders the stats card
- `renderInventory(w, r, charID)` -- re-renders inventory with OOB swaps for encumbrance and companions
- `renderCompanions(w, r, charID)`
- `renderRetainers(w, r, charID)` -- re-renders the retainers section
- `renderNotes(w, r, charID)`
- `renderSheetBody(w, r, charID)` -- re-renders entire sheet body

OOB (Out of Band) swaps allow a single handler response to update multiple independent page sections. For example, adding an item updates the inventory section AND the encumbrance section AND companion sections simultaneously.

## Helper Functions

- `combineStackableItems()` -- Merges stackable items (bundled items sum quantities, coins merge denomination notes) at the same location
- `deductItemQuantity()` -- Removes N of a named item across multiple stacks
- `parseItemInput()` -- Parses "5x preserved rations" into (name, quantity)
- `extractTinyFlag()` -- Parses "tiny lock of hair" into (true, "lock of hair")
- `addToFound()` -- Adjusts found treasure fields on character by coin type and amount
