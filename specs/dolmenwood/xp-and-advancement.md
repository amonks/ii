# XP and Advancement

## XP Calculation (`engine/xp.go`)

### XP Modifiers

`ApplyXPModifiers(base, modPercent)` -- Applies a percentage modifier to base XP: `base + (base * modPercent / 100)`. Uses integer arithmetic.

The total XP modifier comes from `TotalXPModifier(kindred, scores, primes)` in `traits.go`, which combines:
- Prime ability XP modifier (from ability scores, -20% to +10%)
- Kindred XP modifier (Humans get +10%, others 0%)

### Level Detection

`DetectLevelUp(class, currentLevel, xp)` -- Checks if XP qualifies for a higher level for the given class. Returns `(newLevel, didLevelUp)`. Uses `ClassLevelForXP` to parse XP thresholds from the class's advancement table.

`XPToNextLevel(class, currentLevel, currentXP)` -- Returns remaining XP needed to reach the next level for the given class. Uses `ClassXPForLevel`.

### XP Sources

XP is gained from:
1. **Found treasure returned to safety** -- GP value of found treasure, modified by XP percentage modifier
2. **Manual XP grants** -- Direct XP additions via the "Add XP" form

## Advancement Tables (`engine/advancement.go`)

### Data Structure

`AdvancementTable{Title, Headers, Rows}` -- String-based table for display. Headers and rows are string slices.

`AdvancementTableForClass(class)` -- Case-insensitive lookup. Returns `false` for unknown classes.

### Supported Classes (9)

All classes have 15-level advancement tables with:
- Level, XP threshold, Hit Points (die), Attack bonus
- Class-specific columns (Combat Talents for Fighter, Glamours for Enchanter, AC Bonus for Friar)
- Five saving throw targets (Doom, Ray, Hold, Blast, Spell)

Classes: Bard, Cleric, Enchanter, Fighter, Friar, Hunter, Knight, Magician, Thief.

### Column Layout

Every advancement table follows a consistent structure:
- Column 0: Level
- Column 1: XP (comma-formatted, e.g. "2,250")
- Column 2: Hit Points (die expression, e.g. "1d8", "+1d8", "+2")
- Column 3: Attack bonus (e.g. "+1", "+0")
- Columns 4..N-6: Class-specific (optional, varies by class)
- Last 5 columns: Doom, Ray, Hold, Blast, Spell (save targets)

Classes with extra columns between Attack and saves:
- **Fighter**: column 4 = "Combat Talents" (count of talents available)
- **Friar**: column 4 = "AC Bonus" (Armour of Faith bonus)
- **Enchanter**: column 4 = "Glamours" (number of glamours known)

## Generic Class Functions (`engine/class.go`)

These functions parse the string-based advancement tables to extract typed values for any class:

### `ClassAttackBonus(class, level) int`
Parses the Attack column (index 3) from the advancement table. Strips the "+" prefix and converts to integer.

### `ClassSaveTargets(class, level) SaveTargets`
Parses the last 5 columns of the advancement table row for the given level. Returns a `SaveTargets{Doom, Ray, Hold, Blast, Spell}` struct.

### `ClassLevelForXP(class, xp) int`
Parses the XP column (index 1), stripping commas before integer conversion. Iterates all rows to find the highest level whose XP threshold the character meets.

### `ClassXPForLevel(class, level) int`
Returns the XP threshold for a specific level of a class.

### `ClassPrimes(class) []string`
Returns the prime ability score names for a class. When a class has multiple primes, XP modifiers use the lowest score among them.

| Class | Primes |
|-------|--------|
| Knight | str, cha |
| Fighter | str |
| Hunter | con, dex |
| Cleric | wis |
| Friar | int, wis |
| Magician | int |
| Thief | dex |
| Bard | cha, dex |
| Enchanter | cha, int |

### `ClassNames() []string`
Returns all 9 class names in display order.

### `KindredNames() []string`
Returns all 6 kindred names in display order.

### `IsValidClass(class) bool` / `IsValidKindred(kindred) bool`
Case-insensitive validation.

### `ClassSpecificColumns(class, level) map[string]string`
Returns key-value pairs for any class-specific columns between Attack and saves. For example, Fighter at level 6 returns `{"Combat Talents": "2"}`.

## Knight-Specific Functions (Deprecated)

The following functions in `engine/knight.go` are deprecated wrappers that call the generic functions above:

- `KnightLevelForXP(xp)` → calls `ClassLevelForXP("Knight", xp)`
- `KnightAttackBonus(level)` → calls `ClassAttackBonus("Knight", level)`
- `KnightSaveTargets(level)` → calls `ClassSaveTargets("Knight", level)`
- `KnightTraits(level)` → removed; use `ClassTraits("Knight", level)` instead

The `Traits` struct (`MonsterSlayer`, `Knighthood` booleans) is removed. Level-gated Knight features are already covered by `ClassTraits` in `traits.go`.

The `knightTable` programmatic table is removed. The advancement table in `advancement.go` is the single source of truth for all classes.

## Level-Up Flow

### In the View Model

`buildCharacterView()` computes:
- `XPModPercent` -- Total XP modifier percentage (using `ClassPrimes(ch.Class)`)
- `XPToNext` -- XP remaining to next level (using `XPToNextLevel(ch.Class, ...)`)
- `CanLevelUp` -- Whether current XP qualifies for a higher level (using `DetectLevelUp(ch.Class, ...)`)
- `NewLevel` -- The level the character would advance to

### HTTP Handler (`handleLevelUp`)

1. Checks XP threshold via `engine.DetectLevelUp(ch.Class, ch.Level, ch.TotalXP)`
2. If eligible, increments character level
3. Creates audit log entry

### UI

The XP section shows:
- Progress bar toward next level
- Pulsing amber "Level Up" button when eligible (with confirmation dialog)
- XP log (collapsible history of all XP gains)
- Total XP and modifier percentage

## Return to Safety Flow

This is the primary XP-generation mechanic:

1. Character accumulates found treasure during adventures (tracked in `FoundCP/SP/EP/GP/PP` fields)
2. Player clicks "Return to Safety"
3. `db.ReturnToSafety()`:
   - Calculates GP value of found treasure
   - Applies XP modifier percentage (using `ClassPrimes(ch.Class)`)
   - Adds result to `TotalXP`
   - Zeroes found treasure fields
   - Creates XP log and audit entries
