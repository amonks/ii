# Phase 2: Retainer Contracts + Adventurer Retainers

## Goal

Adventurer retainers are full Character records linked to their employer via a `retainer_contracts` table. Employers can hire and dismiss retainers from their character sheet. Retainers appear with full inline stats on the employer's sheet and are accessible via their own standalone sheets.

## Prerequisites

Phase 1 (Generalize Class/Kindred) must be complete — retainers need to be created with any class/kindred.

## Steps

### 1. DB: RetainerContract model and migration

**Modify**: `db/db.go`

Add model:
```go
type RetainerContract struct {
    ID           uint      `gorm:"primarykey"`
    EmployerID   uint      `gorm:"column:employer_id;index"`
    RetainerID   uint      `gorm:"column:retainer_id;index"`
    LootSharePct float64   `gorm:"column:loot_share_pct;default:15.0"`
    XPSharePct   float64   `gorm:"column:xp_share_pct;default:50.0"`
    DailyWageCP  int       `gorm:"column:daily_wage_cp;default:0"`
    HiredOnDay   int       `gorm:"column:hired_on_day;default:1"`
    Active       bool      `gorm:"column:active;default:true"`
    CreatedAt    time.Time
}
```

Add migration:
```go
const migrationRetainerContracts = `
CREATE TABLE IF NOT EXISTS retainer_contracts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    employer_id INTEGER NOT NULL,
    retainer_id INTEGER NOT NULL,
    loot_share_pct REAL NOT NULL DEFAULT 15.0,
    xp_share_pct REAL NOT NULL DEFAULT 50.0,
    daily_wage_cp INTEGER NOT NULL DEFAULT 0,
    hired_on_day INTEGER NOT NULL DEFAULT 1,
    active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_retainer_contracts_employer ON retainer_contracts(employer_id);
CREATE INDEX IF NOT EXISTS idx_retainer_contracts_retainer ON retainer_contracts(retainer_id);
`
```

Add CRUD methods:
- `CreateRetainerContract(rc *RetainerContract) error`
- `ListActiveRetainerContracts(employerID uint) ([]RetainerContract, error)`
- `GetRetainerContract(id uint) (*RetainerContract, error)`
- `UpdateRetainerContract(rc *RetainerContract) error`
- `DeactivateRetainerContract(id uint) error`

Update `DeleteCharacter(id)` to also delete contracts where `employer_id = id` or `retainer_id = id`.

**Tests** (`db/db_test.go`):
- Create contract, verify round-trip
- List active contracts for employer
- Deactivate contract, verify it's excluded from active list
- Delete employer, verify contracts deleted but retainer Character remains
- Delete retainer, verify contracts deleted but employer Character remains

### 2. Server: RetainerView

**Modify**: `server/views.go`

Add struct:
```go
type RetainerView struct {
    Contract      db.RetainerContract
    Character     *db.Character
    AC            int
    AttackBonus   int
    Saves         engine.SaveTargets
    Speed         int
    Loyalty       int
    Weapons       []engine.EquippedWeapon
    KindredTraits []engine.Trait
    ClassTraits   []engine.Trait
}
```

Add field to `CharacterView`:
```go
Retainers []RetainerView
```

In `buildCharacterView`, after building companion views:
1. Call `db.ListActiveRetainerContracts(ch.ID)`
2. For each contract, call `db.GetCharacter(contract.RetainerID)`
3. Load the retainer's items
4. Compute retainer stats using engine functions (ClassAttackBonus, ClassSaveTargets, CharacterAC, EquippedWeapons, etc.)
5. Build `RetainerView` and append to `Retainers` slice

**Tests** (`server/views_test.go`):
- Create employer + retainer + contract, build view, verify retainer stats appear

### 3. Server: Hire handler

**Modify**: `server/handlers.go`, `server/server.go`

Route: `POST /characters/{id}/retainers/`

Handler `handleHireRetainer`:
1. Parse form: name, class, kindred, STR-CHA, hp_max, alignment, loot_share_pct, xp_share_pct, daily_wage_cp
2. Validate class/kindred
3. Create `db.Character` (Level 1, specified stats)
4. Compute loyalty: `engine.RetainerLoyalty(engine.Modifier(employer.CHA))`
5. Create `db.RetainerContract` with employer ID, retainer ID, contract terms, `HiredOnDay = employer.CurrentDay`
6. Audit log on employer: "Hired [name], Level 1 [Kindred] [Class]"
7. Audit log on retainer: "Hired by [employer name]"
8. Re-render retainers section

**Tests** (`server/server_test.go`):
- POST to hire, verify Character and Contract created
- Verify audit logs on both characters
- Verify retainer appears in employer's view

### 4. Server: Dismiss handler

Route: `POST /characters/{id}/retainers/{contractID}/dismiss/`

Handler `handleDismissRetainer`:
1. Load contract, verify employer matches `{id}`
2. Call `db.DeactivateRetainerContract(contractID)`
3. Audit log on both characters
4. Re-render retainers section

**Tests**: Dismiss, verify contract deactivated, retainer Character still exists.

### 5. Server: Update contract handler

Route: `POST /characters/{id}/retainers/{contractID}/update/`

Handler `handleUpdateRetainerContract`:
1. Load contract, verify employer matches
2. Update loot_share_pct, xp_share_pct, daily_wage_cp from form
3. Save contract
4. Re-render retainers section

### 6. Server: Retainers template

**New file**: `server/retainers.templ`

`RetainersSection(view *CharacterView)` renders:

For each retainer in `view.Retainers`:
- Header: name, "Lvl N Kindred Class", link to `/characters/{retainerID}/`
- Stat grid (4 cols): HP (current/max), AC, Atk (+N), Speed
- Saves grid (5 cols): Doom, Ray, Hold, Blast, Spell
- Loyalty display
- Contract terms (editable form): loot share %, XP share %, daily wage
- Dismiss button

"Hire Adventurer" form at the bottom:
- Name, Class (select), Kindred (select)
- Ability scores (6 number inputs)
- HP Max
- Alignment
- Loot Share %, XP Share %, Daily Wage
- Submit button

### 7. Server: Update sheet.templ

**Modify**: `server/sheet.templ`

Add `@RetainersSection(view)` after `@CompanionsSection(view)`.

### 8. Server: Update list.templ

**Modify**: `server/list.templ`

Show retainer characters with a label indicating their employer. Group them under their employer in the list, or add a "(Retainer of [name])" subtitle.

### 9. Server: Add HTMX render helper

**Modify**: `server/handlers.go`

Add `renderRetainers(w, r, ch)` helper that re-renders just the retainers section for HTMX partial updates.

### 10. Generate and test

```
templ generate
go test ./apps/dolmenwood/...
```

## Files Changed

- `db/db.go` (modified — new model, migration, CRUD, cascade update)
- `db/db_test.go` (modified — contract tests)
- `server/views.go` (modified — RetainerView, Retainers field, loading)
- `server/views_test.go` (modified — retainer view tests)
- `server/handlers.go` (modified — hire/dismiss/update handlers, render helper)
- `server/server.go` (modified — new routes)
- `server/retainers.templ` (new)
- `server/sheet.templ` (modified — add retainers section)
- `server/list.templ` (modified — retainer display)
- `server/server_test.go` (modified — handler tests)

## Verification

1. `go test ./apps/dolmenwood/...` passes
2. Hire a retainer from the employer's sheet — verify Character and Contract created
3. Retainer appears in employer's retainers section with correct stats
4. Retainer has own accessible character sheet at `/characters/{id}/`
5. Dismiss a retainer — verify contract deactivated, retainer still exists
6. Update contract terms — verify changes persist
7. Delete employer — verify contracts removed, retainer survives
