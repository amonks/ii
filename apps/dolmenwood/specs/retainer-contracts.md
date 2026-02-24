# Retainer Contracts

## Overview

Adventurer retainers are full `Character` records linked to their employer via a `RetainerContract`. Unlike townsfolk retainers (which are simple `Companion` records), adventurer retainers have their own character sheets with class, kindred, ability scores, inventory, XP, and advancement.

## Rules Summary

Per the Dolmenwood rules:
- A PC can employ a maximum of **4 + CHA modifier** retainers at a time (townsfolk + adventurers combined)
- Adventurer retainers start at Level 1 with a randomly determined class
- They are counted as **party members for XP division**
- All earned XP is **halved** by default (configurable via contract)
- They can **advance in level** just like PCs
- They typically leave employment when reaching equal or greater level than their employer
- They can be promoted to full PCs if a player character dies
- Initial loyalty = **7 + employer's CHA modifier**

## Data Model (`db/db.go`)

### RetainerContract

```
RetainerContract {
    ID           uint
    EmployerID   uint      // FK to characters
    RetainerID   uint      // FK to characters
    LootSharePct float64   // e.g. 15.0 for 15%
    XPSharePct   float64   // e.g. 50.0 for half XP (default)
    DailyWageCP  int       // daily wage in copper pieces
    HiredOnDay   int       // game day when hired
    Active       bool      // whether contract is currently active
    CreatedAt    time.Time
}
```

### DB Methods

- `CreateRetainerContract(rc)` -- creates a new contract
- `ListActiveRetainerContracts(employerID)` -- returns active contracts for an employer
- `GetRetainerContract(id)` -- fetches a single contract
- `UpdateRetainerContract(rc)` -- updates contract terms (loot share, XP share, daily wage)
- `DeactivateRetainerContract(id)` -- sets `active = false` (dismissal)

### Cascade Behavior

When deleting a character:
- Contracts where the character is the employer are deleted (retainer Characters remain)
- Contracts where the character is the retainer are deleted (employer Characters remain)

## Hiring Flow

From the employer's character sheet:

1. Player fills in the "Hire Adventurer" form: retainer name, class, kindred, ability scores, HP, alignment, contract terms (loot share %, XP share %, daily wage)
2. `handleHireRetainer` handler:
   - Validates class and kindred
   - Creates a new `Character` record (Level 1, with specified stats)
   - Computes initial loyalty: `7 + Modifier(employer.CHA)`
   - Creates a `RetainerContract` linking employer and retainer
   - Creates audit log entries on both characters
3. The retainer appears in the employer's sheet and is accessible via their own URL

## Dismissal Flow

1. Player clicks "Dismiss" on a retainer
2. `handleDismissRetainer` handler:
   - Sets `contract.Active = false`
   - Creates audit log entries on both characters
3. The retainer Character persists as an independent character (could be re-hired)

## Contract Updates

Contract terms (loot share %, XP share %, daily wage) can be updated via `handleUpdateRetainerContract`. This allows renegotiation without dismissal.

## Retainer on Employer's Sheet

### RetainerView (`server/views.go`)

For each active contract, `buildCharacterView` constructs a `RetainerView` with:
- **Contract terms**: loot share %, XP share %, daily wage
- **Character data**: name, class, kindred, level, ability scores
- **Computed combat stats**: AC, attack bonus, saves, speed, weapons (same engine functions as PCs)
- **Traits**: kindred traits + class traits
- **Loyalty**: from contract (initially 7 + employer CHA mod)
- **Full inventory**: retainer's items, built into an inventory tree using the same `buildInventoryTree` function

### UI Layout (`server/retainers.templ`)

The retainers section appears after companions on the employer's sheet. For each retainer:

1. **Header**: Name, "Lvl N Kindred Class", link to full sheet
2. **Stat grid**: HP (editable), AC, Attack, Speed
3. **Saves**: Doom, Ray, Hold, Blast, Spell (+ magic resistance + conditional bonuses)
4. **Loyalty/Morale display**
5. **Contract terms**: loot share, XP share, daily wage (editable)
6. **Inventory**: full tree with transfer buttons (give/take items)
7. **Actions**: Dismiss button

### Character List

On the index page, retainer characters appear indented under their employer (or with a "(Retainer of X)" label). They are also accessible independently via their own character sheet URL.

## Retainer as Standalone Character

An adventurer retainer's own character sheet is a fully functional character sheet — identical to any PC's sheet. It shows their own stats, inventory, XP, advancement table, traits, notes, etc. Some sessions a player may choose to play as their retainer instead of their main character.

The retainer's sheet does NOT show their contract terms or employer relationship — that information lives on the employer's sheet.
