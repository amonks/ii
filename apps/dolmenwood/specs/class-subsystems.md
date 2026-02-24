# Class-Specific Mechanical Subsystems

## Overview

Each of the 9 classes has unique mechanical subsystems beyond basic stats (attack, saves, HP). This spec describes the target state for full mechanical support of all class-specific features.

## Spellcasting (Cleric, Friar, Magician, Enchanter)

### Spell Slots

Four classes have spell slots that scale with level. Slot counts come from the Dolmenwood rules (not currently in the advancement tables):

`ClassSpellSlots(class, level) *SpellSlots` returns available spell slots per spell level (1-6).

- **Cleric**: Holy spells from level 2+. Slots scale from 1 first-level slot at L2 to multiple high-level slots at L15.
- **Friar**: Holy spells from level 1 (if faith maintained). Similar progression to Cleric but starting earlier.
- **Magician**: Arcane spells from level 1. Broadest spell progression.
- **Enchanter**: Arcane spells via glamours. Count of glamours known is in the advancement table ("Glamours" column).

### Spell Tracking

**DB Model**: `prepared_spells` table:
```
PreparedSpell {
    ID          uint
    CharacterID uint
    Name        string
    SpellLevel  int
    Used        bool
}
```

**Engine Functions**:
- `ClassSpellSlots(class, level)` -- available slots per spell level
- `AvailableSlots(slots, prepared)` -- remaining slots after accounting for prepared spells

**Handlers**:
- `POST /characters/{id}/spells/` -- prepare a spell (add to prepared list)
- `POST /characters/{id}/spells/{spellID}/cast/` -- mark a spell as used
- `POST /characters/{id}/spells/rest/` -- reset all used spells (long rest)
- `POST /characters/{id}/spells/{spellID}/forget/` -- remove a prepared spell

**UI** (`server/spells.templ`):
- Only shown for spellcasting classes
- Spell slot grid showing available / total per level
- Prepared spell list with "Cast" (marks used) and "Forget" buttons
- "Prepare Spell" form (name + level)
- "Rest" button to reset all used spells

## Combat Talents (Fighter)

The Fighter gains combat talents at levels 2, 6, 10, and 14 (from advancement table "Combat Talents" column). There are 8 possible talents: Battle Rage, Cleave, Defender, Last Stand, Leader, Main Gauche, Slayer, Weapon Specialist.

**Implementation**: Display the number of talents available from `ClassSpecificColumns("Fighter", level)["Combat Talents"]`. Track chosen talents via the existing notes system or a simple text list.

**UI**: A "Combat Talents" section in the Fighter's traits area showing:
- Number of talents available at current level
- List of chosen talents (editable text)
- Descriptions of each talent's effects

## Backstab (Thief)

The Thief's backstab ability: +4 to attack, 3d4 damage when striking an unaware target from behind.

**Implementation**: Display backstab stats in the combat section. The damage multiplier scales with level in some variants, but in Dolmenwood it's fixed at 3d4.

**UI**: Show backstab bonus and damage in the combat stats area, alongside normal weapons.

## Thief Skills

Thief skill target numbers improve by level. Skills: Climb, Disarm Traps, Hear Noise, Hide in Shadows, Move Silently, Open Locks, Pick Pockets, Read Languages.

**Implementation**: `ThiefSkillTargets(level) map[string]int` returns target numbers for each skill at the given level.

**UI**: A "Thief Skills" section showing each skill with its target number.

## Animal Companion (Hunter)

Hunters bond with an animal companion at level 1. This is a special companion with its own stats, HP, and abilities.

**Implementation**: The Hunter's animal companion can be modeled as a `Companion` record with a special breed type (e.g., "Animal Companion") whose stats are entered manually rather than derived from a breed catalog. Alternatively, extend the companion system with custom stat entry.

**UI**: Similar to the existing companion section, but with editable stats (since animal companions vary).

## Glamours (Enchanter, Elf, Grimalkin, Woodgrue)

Glamours are fairy magic abilities. Enchanters learn glamours by level (count from advancement table). Elves, Grimalkin, and Woodgrue know one glamour from their kindred.

**Implementation**: Track glamour names as a list. The count of available glamours for Enchanters comes from `ClassSpecificColumns("Enchanter", level)["Glamours"]`.

**UI**: A "Glamours" section listing known glamours. For Enchanters, show available count vs. known count.

## Turning Undead (Cleric, Friar)

Clerics and Friars can attempt to turn (repel or destroy) undead creatures. Success depends on the undead's type and the character's level, using a Turn Undead table.

**Implementation**: `TurnUndeadTarget(class, level, undeadHD) string` returns the target number (or "T" for automatic turn, "D" for automatic destroy, "-" for impossible).

**UI**: A "Turn Undead" table showing targets by undead HD at the character's level.

## Armour of Faith (Friar)

Friars gain an AC bonus by level (from advancement table "AC Bonus" column). This stacks with DEX but not with worn armor.

**Implementation**: Integrate into `CharacterAC` calculation. When class is Friar and no armor is worn, add the AC bonus from `ClassSpecificColumns("Friar", level)["AC Bonus"]`.

**UI**: AC display already handles this — just needs the engine to include the bonus.

## Herbalism (Friar)

Friars can identify and prepare herbal remedies. Healing herbs restore double HP when prepared by a Friar.

**Implementation**: Display as a trait. Mechanical tracking (herb inventory, double healing) can be handled via the existing item/notes system.

## Bard Enchantment & Counter Charm

- **Enchantment**: Uses per day equal to the Bard's level. Enchanted performances affect listeners.
- **Counter Charm**: Neutralize enchanted music within 30'.

**Implementation**: Track enchantment uses per day (total = level, reset on rest). Counter Charm is passive/reactive — display as a trait.

**UI**: Show enchantment uses remaining / total. "Use Enchantment" button. "Rest" resets uses.

## Bard Skills

Decipher Script and Monster Lore with skill targets by level.

**Implementation**: `BardSkillTargets(level) map[string]int` returns targets.

**UI**: Skill target display similar to Thief Skills.

## Implementation Priority

These subsystems are independent and can be implemented in any order. Suggested priority based on frequency of use:

1. **Spell system** (Cleric, Friar, Magician, Enchanter) — most mechanically complex, affects 4 classes
2. **Friar Armour of Faith** — affects AC calculation, important for correctness
3. **Thief Skills + Backstab** — simple display, high play value
4. **Combat Talents (Fighter)** — simple tracking
5. **Glamours** — affects Enchanter + 3 kindreds
6. **Turn Undead** — table display
7. **Bard Enchantment** — uses/day tracking
8. **Animal Companion (Hunter)** — custom companion entry
9. **Bard/Hunter Skill Targets** — simple display
10. **Herbalism** — trait display, no special mechanics needed
