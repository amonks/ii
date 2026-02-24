# Phase 1: Generalize Class/Kindred Support

## Goal

Remove all Knight/Human hardcoding. Make all 9 classes and 6 kindreds fully functional for character creation and the character sheet.

## Prerequisites

None.

## Steps

### 1. Engine: Generic class functions

**New files**: `engine/class.go`, `engine/class_test.go`

Write tests first for each function, then implement.

**Functions to implement** (all parse from existing advancement tables in `engine/advancement.go`):

- `ClassAttackBonus(class string, level int) int`
  - Parse column 3 of advancement table, strip "+" prefix
  - Test: verify against known values for each class at several levels

- `ClassSaveTargets(class string, level int) SaveTargets`
  - Parse last 5 columns of advancement table
  - Test: verify against known values for each class

- `ClassLevelForXP(class string, xp int) int`
  - Parse column 1, strip commas before int conversion
  - Iterate rows to find highest qualifying level
  - Test: boundary cases at each level threshold

- `ClassXPForLevel(class string, level int) int`
  - Parse column 1 for the given level row
  - Test: verify thresholds for each class

- `ClassPrimes(class string) []string`
  - Static map: Knight→[str,cha], Fighter→[str], Hunter→[con,dex], Cleric→[wis], Friar→[int,wis], Magician→[int], Thief→[dex], Bard→[cha,dex], Enchanter→[cha,int]
  - Test: all 9 classes

- `ClassNames() []string`, `KindredNames() []string`
  - Ordered lists for UI dropdowns
  - Test: correct count and order

- `IsValidClass(class string) bool`, `IsValidKindred(kindred string) bool`
  - Case-insensitive
  - Test: valid and invalid inputs

- `ClassSpecificColumns(class string, level int) map[string]string`
  - Return columns between Attack (index 3) and saves (last 5)
  - Fighter: "Combat Talents", Friar: "AC Bonus", Enchanter: "Glamours"
  - Classes without extra columns return empty map
  - Test: each class with extra columns, classes without

**Helper**: `parseXPValue(s string) int` -- strips commas, parses int

### 2. Engine: Update xp.go signatures

**Modify**: `engine/xp.go`, `engine/xp_test.go`

- `DetectLevelUp(currentLevel, xp)` → `DetectLevelUp(class string, currentLevel, xp int) (int, bool)`
  - Replace `KnightLevelForXP(xp)` with `ClassLevelForXP(class, xp)`
  - Update all test cases to pass class name

- `XPToNextLevel(currentLevel, currentXP)` → `XPToNextLevel(class string, currentLevel, currentXP int) int`
  - Replace `knightTable[currentLevel].XPRequired` with `ClassXPForLevel(class, currentLevel+1)`
  - Update all test cases

### 3. Engine: Deprecate Knight-specific functions

**Modify**: `engine/knight.go`

- `KnightLevelForXP` → wrapper calling `ClassLevelForXP("Knight", xp)`
- `KnightAttackBonus` → wrapper calling `ClassAttackBonus("Knight", level)`
- `KnightSaveTargets` → wrapper calling `ClassSaveTargets("Knight", level)`
- Remove `KnightTraits` function and `Traits` struct (replaced by `ClassTraits`)
- Remove `knightTable` var (advancement table is the single source of truth)

**Important**: The `knightTable` save values differ from the advancement table at some levels (e.g., level 3). The advancement table is authoritative. Existing tests that used `knightTable` values may need updating.

### 4. Server: Update views.go

**Modify**: `server/views.go`

At line 251:
```
// Before: primes := []string{"str"}
// After:
primes := engine.ClassPrimes(ch.Class)
```

At lines 408-412:
```
// Before:
AttackBonus: engine.KnightAttackBonus(ch.Level),
Saves:       engine.KnightSaveTargets(ch.Level),
Traits:      engine.KnightTraits(ch.Level),

// After:
AttackBonus: engine.ClassAttackBonus(ch.Class, ch.Level),
Saves:       engine.ClassSaveTargets(ch.Class, ch.Level),
// Remove Traits field entirely
```

At line 362:
```
// Before: newLevel, canLevelUp := engine.DetectLevelUp(ch.Level, ch.TotalXP)
// After:
newLevel, canLevelUp := engine.DetectLevelUp(ch.Class, ch.Level, ch.TotalXP)
```

At line 432:
```
// Before: XPToNext: engine.XPToNextLevel(ch.Level, ch.TotalXP),
// After:
XPToNext: engine.XPToNextLevel(ch.Class, ch.Level, ch.TotalXP),
```

Remove `Traits engine.Traits` field from `CharacterView` struct.

### 5. Server: Update stats.templ

**Modify**: `server/stats.templ` (lines 167-176)

Replace Knight-specific badge display:
```templ
// Before:
if view.Traits.Knighthood || view.Traits.MonsterSlayer {
    @Cluster() {
        if view.Traits.Knighthood { @BadgeWarning() { Knighthood } }
        if view.Traits.MonsterSlayer { @BadgeDanger() { Monster Slayer } }
    }
}

// After: Remove this block entirely.
// Level-gated traits are already displayed in the Traits section
// via ClassTraits, which includes Knighthood and Monster Slayer
// conditionally based on level.
```

### 6. Server: Update handlers.go

**Modify**: `server/handlers.go`

At lines 39-40 (handleCreateCharacter):
```
// Before:
Class:   "Knight",
Kindred: "Human",

// After:
Class:   r.FormValue("class"),
Kindred: r.FormValue("kindred"),
```
Add validation: if `!engine.IsValidClass(class)` or `!engine.IsValidKindred(kindred)`, return 400.

At line 915 (handleReturnToSafety):
```
// Before: xpMod := engine.TotalXPModifier(ch.Kindred, scores, []string{"str"})
// After:
xpMod := engine.TotalXPModifier(ch.Kindred, scores, engine.ClassPrimes(ch.Class))
```

At line 934 (handleLevelUp):
```
// Before: newLevel, canLevel := engine.DetectLevelUp(ch.Level, ch.TotalXP)
// After:
newLevel, canLevel := engine.DetectLevelUp(ch.Class, ch.Level, ch.TotalXP)
```

### 7. Server: Update list.templ

**Modify**: `server/list.templ`

Add class and kindred `<select>` elements to the "New Character" form. The option lists come from `engine.ClassNames()` and `engine.KindredNames()`. Pass these to the template via the index handler (or hardcode in the template if simpler).

### 8. Server: Update index handler

**Modify**: `server/handlers.go` (handleIndex)

If needed, pass class/kindred lists to the list template. Alternatively, the template can call the engine functions directly since templ has access to Go packages.

### 9. Generate and test

```
templ generate
go test ./apps/dolmenwood/...
```

Update any broken tests in `server/views_test.go` and `server/server_test.go` (they reference the old `Traits` field and Knight-only function signatures).

## Files Changed

- `engine/class.go` (new)
- `engine/class_test.go` (new)
- `engine/xp.go` (modified)
- `engine/xp_test.go` (modified)
- `engine/knight.go` (modified — deprecate functions, remove Traits/knightTable)
- `engine/knight_test.go` (modified — update expectations)
- `server/views.go` (modified)
- `server/stats.templ` (modified)
- `server/list.templ` (modified)
- `server/handlers.go` (modified)
- `server/views_test.go` (modified)
- `server/server_test.go` (modified)

## Verification

1. `go test ./apps/dolmenwood/...` passes
2. Create a character with each of the 9 classes — verify correct attack bonus, saves, and advancement table
3. Create a character with each of the 6 kindreds — verify correct traits and XP modifier
4. Level up a non-Knight character — verify correct level detection
5. Return to Safety with a non-Knight character — verify correct XP modifier
