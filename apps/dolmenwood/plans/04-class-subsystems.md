# Phase 4: Class-Specific Mechanical Subsystems

## Goal

Full mechanical support for all 9 class subsystems. Each subsystem is independent and can be implemented in any order.

## Prerequisites

Phase 1 (Generalize Class/Kindred) must be complete. Phases 2-3 are not required.

## Subsystems by Priority

### 4a. Spell System (Cleric, Friar, Magician, Enchanter)

The most mechanically complex subsystem, affecting 4 classes.

**Engine** (`engine/spells.go`, `engine/spells_test.go`):

```go
type SpellSlots struct {
    Level1, Level2, Level3, Level4, Level5, Level6 int
}

func ClassSpellSlots(class string, level int) *SpellSlots
```

Returns nil for non-spellcasting classes. Spell slot data comes from the Dolmenwood rules (not in current advancement tables — must be added as static data).

Spell slot progression needs to be transcribed from the rules for:
- Cleric (holy spells from level 2)
- Friar (holy spells from level 1)
- Magician (arcane spells from level 1)
- Enchanter (glamours — count is in advancement table, but spell-like slots may differ)

**DB** (`db/db.go`):

New model:
```go
type PreparedSpell struct {
    ID          uint   `gorm:"primarykey"`
    CharacterID uint   `gorm:"column:character_id;index"`
    Name        string `gorm:"column:name"`
    SpellLevel  int    `gorm:"column:spell_level"`
    Used        bool   `gorm:"column:used"`
}
```

Migration:
```sql
CREATE TABLE IF NOT EXISTS prepared_spells (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    spell_level INTEGER NOT NULL DEFAULT 1,
    used INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_prepared_spells_character ON prepared_spells(character_id);
```

CRUD:
- `ListPreparedSpells(characterID)` — returns all prepared spells
- `CreatePreparedSpell(spell)` — prepare a spell
- `MarkSpellUsed(spellID)` — set used = true
- `ResetSpells(characterID)` — set all used = false (rest)
- `DeletePreparedSpell(spellID)` — forget a spell

Cascade: `DeleteCharacter` also deletes prepared spells.

**Server**:

New routes:
- `POST /characters/{id}/spells/` — prepare a spell
- `POST /characters/{id}/spells/{spellID}/cast/` — mark used
- `POST /characters/{id}/spells/{spellID}/forget/` — delete
- `POST /characters/{id}/spells/rest/` — reset all used

View: Add `SpellSlots *engine.SpellSlots` and `PreparedSpells []db.PreparedSpell` to `CharacterView`. Only populated for spellcasting classes.

Template (`server/spells.templ`):
- Only rendered for spellcasting classes
- Slot grid: per spell level, show used/available
- Prepared spell list with Cast and Forget buttons
- "Prepare Spell" form: name + level select
- "Rest" button

Add `@SpellsSection(view)` to `sheet.templ` (conditionally rendered).

**Files**: `engine/spells.go`, `engine/spells_test.go`, `db/db.go`, `db/db_test.go`, `server/views.go`, `server/handlers.go`, `server/server.go`, `server/spells.templ`, `server/sheet.templ`

---

### 4b. Friar Armour of Faith

Friars gain an AC bonus by level from the advancement table "AC Bonus" column.

**Engine** (`engine/equipment.go` or `engine/class.go`):

Update `CharacterAC` to accept class and level parameters. When class is "Friar" and no armor is worn, add the AC bonus from `ClassSpecificColumns("Friar", level)["AC Bonus"]`.

**Files**: `engine/equipment.go`, `engine/equipment_test.go`, `server/views.go`

---

### 4c. Thief Backstab + Skills

**Engine** (`engine/thief.go`, `engine/thief_test.go`):

```go
func ThiefBackstabDamage() string  // returns "3d4" (fixed in Dolmenwood)
func ThiefBackstabBonus() int      // returns +4

type SkillTargets map[string]int

func ThiefSkillTargets(level int) SkillTargets
```

Skill target numbers by level for: Climb, Disarm Traps, Hear Noise, Hide in Shadows, Move Silently, Open Locks, Pick Pockets, Read Languages.

**Server**: Add skill targets to `CharacterView` (only for Thieves). Display in a skills section.

**Files**: `engine/thief.go`, `engine/thief_test.go`, `server/views.go`, `server/stats.templ` or new `server/skills.templ`

---

### 4d. Fighter Combat Talents

Count of available talents comes from `ClassSpecificColumns("Fighter", level)["Combat Talents"]`.

**Server**: Display talent count. Track chosen talents via freeform text (like notes) or a simple list.

Simplest implementation: show the count in the traits section, let the player use the Notes feature to record their choices.

**Files**: `server/views.go`, `server/traits.templ` (or `server/stats.templ`)

---

### 4e. Glamours (Enchanter, Elf, Grimalkin, Woodgrue)

Enchanters learn glamours by level (count from advancement table). Elf, Grimalkin, and Woodgrue know one glamour from kindred traits.

**Implementation options**:
- Simple: show count of glamours known, let player record names in Notes
- Rich: new `known_glamours` table with name field, UI for adding/removing

Start with the simple approach — display the count from `ClassSpecificColumns("Enchanter", level)["Glamours"]` and note that kindred glamours come from trait descriptions.

**Files**: `server/views.go`, `server/traits.templ`

---

### 4f. Turn Undead (Cleric, Friar)

**Engine** (`engine/turn_undead.go`, `engine/turn_undead_test.go`):

```go
func TurnUndeadTable(class string, level int) []TurnUndeadEntry

type TurnUndeadEntry struct {
    UndeadHD string  // "1", "2", "3", etc. or "Special"
    Target   string  // number, "T" (auto turn), "D" (auto destroy), "-" (impossible)
}
```

The turn undead table values are derived from the Dolmenwood cleric/friar rules (2d6-based B/X table).

**Server**: Display as a table in the character sheet (only for Clerics and Friars).

**Files**: `engine/turn_undead.go`, `engine/turn_undead_test.go`, `server/views.go`, new section in `server/stats.templ` or `server/spells.templ`

---

### 4g. Bard Enchantment + Counter Charm

Enchantment uses per day = Bard's level. Counter Charm is a passive ability.

**Implementation**: Track enchantment uses per day. Could use the same pattern as spell tracking (prepare/cast/rest) but simpler — just a counter.

Simplest: add `EnchantmentUsesTotal` and `EnchantmentUsesRemaining` fields to the view (computed from level). Use the same rest mechanism as spells to reset.

Or even simpler: display "Enchantment: N uses/day" as a trait, rely on player tracking.

**Files**: `server/views.go`, `server/traits.templ`

---

### 4h. Hunter Animal Companion

Hunters bond with an animal companion. Unlike mounts, these have custom stats.

**Implementation**: Model as a `Companion` record. The existing companion system supports custom HP and name. For the animal companion, we may need:
- Custom AC, attack, and speed (not breed-derived)
- A way to mark a companion as the Hunter's bonded animal

Simplest approach: add a "Custom" breed that allows manual stat entry. Or add fields to the Companion model for override stats.

**Files**: `engine/companions.go`, `db/db.go`, `server/companions.templ`

---

### 4i. Bard/Hunter Skill Targets

**Engine**:
```go
func BardSkillTargets(level int) SkillTargets  // Decipher Script, Monster Lore
func HunterSkillTargets(level int) SkillTargets // Alertness, Stalking, Tracking
```

**Server**: Display skill targets in a skills section (shared with Thief skills).

**Files**: `engine/bard.go` or `engine/skills.go`, `server/views.go`, `server/skills.templ`

---

### 4j. Friar Herbalism

Display as a trait. The "double healing from herbs" mechanic is informational — no special tracking needed beyond what the trait description provides.

**Files**: Already covered by `ClassTraits("Friar", level)` in `traits.go`.

## Implementation Order

Each subsystem is independent. Suggested order:

1. Spell system (4a) — most impact, most complex
2. Friar Armour of Faith (4b) — affects AC correctness
3. Thief Backstab + Skills (4c) — simple, high play value
4. Fighter Combat Talents (4d) — simple display
5. Turn Undead (4f) — table display
6. Glamours (4e) — count display
7. Bard Enchantment (4g) — uses tracking
8. Bard/Hunter Skills (4i) — simple display
9. Hunter Animal Companion (4h) — companion system extension
10. Herbalism (4j) — already covered by traits

## Verification

For each subsystem:
1. `go test ./apps/dolmenwood/...` passes
2. Create a character of the relevant class
3. Verify the class-specific section appears with correct data
4. For interactive features (spells, enchantment), verify the prepare/cast/rest cycle works
5. For display-only features (skills, backstab, turn undead), verify correct values at multiple levels
