# Item Transfers

## Overview

Items can be transferred between a character and their adventurer retainers. The retainer's items always belong to the retainer's `Character` record (single source of truth). The employer's sheet displays the retainer's inventory inline and provides UI for transferring items in both directions.

## Data Model

Items are always owned by exactly one character via `item.CharacterID`. A transfer changes this ownership:

### `db.TransferItem(itemID, toCharacterID, quantity)`

- **Full transfer** (quantity equals item.Quantity or quantity is 0): Updates `item.CharacterID` to the target. Clears `ContainerID` and `CompanionID` (item lands in the target's equipped/root inventory).
- **Partial transfer** (quantity < item.Quantity): Creates a new item on the target character with the specified quantity and same properties (name, notes, IsTiny, WeightOverride). Reduces the source item's quantity by the transferred amount.

Special considerations:
- Coin items: the transfer operation handles splitting coin denomination notes appropriately
- Container items with children: transferring a container moves its children too (updating their CharacterID)

## Retainer Inventory on Employer's Sheet

### How It Appears

In `buildCharacterView`, for each active retainer contract:
1. Load the retainer's `Character` record
2. Load all the retainer's `Item` records
3. Build the retainer's inventory tree using the same `buildInventoryTree` function
4. Include the tree in the `RetainerView`

The retainer's inventory appears within the retainer's section on the employer's sheet, showing:
- Equipped items on the retainer
- Items in the retainer's containers
- Items on the retainer's own companions (if any)
- Used slots / capacity indicators
- An add-item form that creates items directly on the retainer

### Transfer UI

Each item in the retainer's inventory (when viewed from the employer's sheet) has a "Take" button that transfers it to the employer. The employer's inventory has a "Give to [retainer name]" move target option. The retainer inventory includes an add-item form that posts to `POST /characters/{id}/retainers/{contractID}/items/` and uses the same item parsing rules as the employer add-item flow.

Transfer form values:
- `item_id`: the item to transfer
- `quantity`: how many to transfer (defaults to all)
- `direction`: `"give"` (employer → retainer) or `"take"` (retainer → employer)

### Move Target Integration

Adventurer retainers are added to the employer's move target dropdown list. The format is `"retainer:{contractID}"` as the move target value. When an item is moved to a retainer target:
1. `handleUpdateItem` recognizes the `retainer:` prefix
2. Calls `db.TransferItem` to move the item to the retainer's character
3. Re-renders both the inventory and retainers sections

The move-to dropdown is only used from the employer's inventory; retainer items are transferred back using the "Take" buttons on each retainer item row.

## Audit Trail

Every transfer creates audit log entries on **both** characters:
- On the source: "Gave [item name] (×N) to [target name]"
- On the target: "Received [item name] (×N) from [source name]"

## Encumbrance Impact

After a transfer:
- The source character's encumbrance decreases (items removed)
- The target character's encumbrance increases (items added)
- Both characters' speed may change as a result
- The employer's sheet re-renders to reflect updated encumbrance on both the employer and the retainer
