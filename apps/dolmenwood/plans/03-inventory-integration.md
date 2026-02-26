# Phase 3: Inventory Integration + Item Transfers

## Goal

The employer's sheet shows each retainer's full inventory inline. Items can be transferred between employer and retainer characters. Move targets include retainers.

## Prerequisites

Phase 2 (Retainer Contracts) must be complete.

## Steps

### 1. DB: TransferItem method

**Modify**: `db/db.go`

```go
func (db *DB) TransferItem(itemID uint, toCharacterID uint, quantity int) error
```

Logic:
1. Load the item
2. If `quantity == 0` or `quantity >= item.Quantity`: full transfer
   - Update `item.CharacterID = toCharacterID`
   - Clear `item.ContainerID = nil`, `item.CompanionID = nil`
   - If the item is a container, also update all child items (items with `ContainerID = itemID`) to have `CharacterID = toCharacterID`
   - If the source character's coin location references this coin item or its container/companion, clear those references
   - Save
3. If `quantity < item.Quantity`: partial transfer
   - Reduce `item.Quantity` by `quantity`, save
   - Create a new item on the target character: same Name, Notes, IsTiny, WeightOverride, with `Quantity = quantity`, `ContainerID = nil`, `CompanionID = nil`

**Tests** (`db/db_test.go`):
- Full transfer: item moves, CharacterID updated, ContainerID/CompanionID cleared
- Partial transfer: original item quantity reduced, new item created on target
- Container transfer: children follow the container
- Transfer with coin item: handles denomination notes correctly
- Verify source character's encumbrance decreases (by checking items)

### 2. Server: Expand RetainerView with inventory

**Modify**: `server/views.go`

Add to `RetainerView`:
```go
Items           []db.Item
EquippedItems   []InventoryItem
CompanionGroups []CompanionInventory
EquippedSlots   int
StowedSlots     int
StowedCapacity  int
MoveTargets     []MoveTarget  // move targets within the retainer's inventory
```

In `buildCharacterView`, for each retainer:
1. Load retainer's items: `db.ListItems(retainer.ID)`
2. Load retainer's companions: `db.ListCompanions(retainer.ID)`
3. Convert items to engine items
4. Calculate encumbrance: `engine.CalculateEncumbrance(engineItems)`
5. Build companion views for retainer's companions
6. Build inventory tree: reuse `buildInventoryTree(items, compViews, companionSlots)`
7. Compute speed from slots
8. Populate RetainerView fields

### 3. Server: Transfer handler

**Modify**: `server/handlers.go`, `server/server.go`

Route: `POST /characters/{id}/retainers/{contractID}/transfer/`

Handler `handleTransferItem`:
1. Parse form: `item_id`, `quantity`, `direction` ("give" or "take")
2. Load contract, verify employer matches `{id}` and contract is active
3. Determine source/target:
   - "give": source = employer, target = retainer
   - "take": source = retainer, target = employer
4. Verify the item belongs to the source character
5. Call `db.TransferItem(itemID, targetCharacterID, quantity)`
6. Audit log on employer: "Gave/Received [item] to/from [retainer name]"
7. Audit log on retainer: "Received/Gave [item] from/to [employer name]"
8. Re-render inventory and retainers sections (OOB swap)

**Tests** (`server/server_test.go`):
- Give item to retainer: item moves to retainer's Character
- Take item from retainer: item moves to employer's Character
- Partial transfer: stack split correctly
- Invalid direction: returns error
- Item not owned by source: returns error

### 4. Server: Add retainers to move targets

**Modify**: `server/views.go`

In `buildMoveTargets`, after adding companion targets, add each active retainer:
```go
for _, ret := range retainers {
    targets = append(targets, MoveTarget{
        Value: fmt.Sprintf("retainer:%d", ret.Contract.ID),
        Label: fmt.Sprintf("→ %s (%s)", ret.Character.Name, ret.Character.Class),
    })
}
```

**Modify**: `server/handlers.go`

In `handleUpdateItem`, add handling for `move_to` values with `retainer:` prefix:
1. Parse contract ID from the value
2. Load contract, verify employer
3. Call `db.TransferItem(itemID, contract.RetainerID, 0)` (full transfer)
4. Audit logs
5. Re-render

### 5. Server: Update retainers.templ with inventory

**Modify**: `server/retainers.templ`

For each retainer, after the stat block, add:

Inventory section:
- Equipped items list (reuse `inventoryItem` component from `inventory.templ`)
- Companion inventory groups (if retainer has companions)
- Each item has a "Take" button (form posting to transfer endpoint with direction=take)
- Slots used / capacity indicator
- "Add item" form that creates items directly on the retainer's Character

Below the employer's regular inventory, show "Give to [retainer]" as a move target option in the MoveSelect dropdown.

### 6. Server: HTMX render updates

**Modify**: `server/handlers.go`

Update `renderInventory` and `renderRetainers` to work together for OOB swaps. After a transfer:
- The inventory section needs to re-render (employer's items changed)
- The retainers section needs to re-render (retainer's items changed)
- Encumbrance sections need to re-render (both characters' slots changed)

### 7. Generate and test

```
templ generate
go test ./apps/dolmenwood/...
```

## Files Changed

- `db/db.go` (modified — TransferItem method)
- `db/db_test.go` (modified — transfer tests)
- `server/views.go` (modified — expanded RetainerView, inventory loading, move targets)
- `server/views_test.go` (modified — retainer inventory tests)
- `server/handlers.go` (modified — transfer handler, move target handling, OOB updates)
- `server/server.go` (modified — transfer route)
- `server/retainers.templ` (modified — inventory display, transfer buttons)
- `server/inventory.templ` (possibly modified — if shared components need extraction)
- `server/server_test.go` (modified — transfer tests)

## Verification

1. `go test ./apps/dolmenwood/...` passes
2. Retainer's inventory appears inline on employer's sheet
3. "Give" an item from employer to retainer — item appears in retainer's inventory, disappears from employer's
4. "Take" an item from retainer — item appears in employer's inventory
5. Partial stack transfer works (e.g., give 3 of 5 torches)
6. Move an item to a retainer via the move-to dropdown
7. Encumbrance updates correctly on both characters after transfer
8. Audit logs appear on both characters
9. Retainer's own sheet also reflects the transferred items
