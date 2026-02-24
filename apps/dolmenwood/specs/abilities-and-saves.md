# Ability Scores, Saves, and Magic Resistance

## Ability Score Modifiers (`engine/abilities.go`)

The classic B/X modifier table maps ability scores (3-18) to modifiers (-3 to +3):

| Score | Modifier |
|-------|----------|
| 3     | -3       |
| 4-5   | -2       |
| 6-8   | -1       |
| 9-12  | 0        |
| 13-15 | +1       |
| 16-17 | +2       |
| 18    | +3       |

`Modifier(score int) int` implements this lookup.

## Prime Ability XP Modifier (`engine/abilities.go`)

A separate bracket table determines XP modifiers based on a class's prime ability scores:

| Score | XP Modifier |
|-------|-------------|
| 3-5   | -20%        |
| 6-8   | -10%        |
| 9-12  | 0%          |
| 13-15 | +5%         |
| 16-18 | +10%        |

`PrimeAbilityXPModifier(scores, primes)` -- when a class has multiple prime abilities, the **lowest** modifier among them is used. Prime abilities per class are returned by `ClassPrimes(class)` in `engine/class.go`.

## Armor Class (`engine/abilities.go`, `engine/equipment.go`)

### `ACFromArmor(baseAC, dexScore, hasShield)`
Base formula: `baseAC + Modifier(dexScore) + (1 if shield)`

### `CharacterAC(kindred, items, dexScore)` (in `equipment.go`)
Scans equipped items for the best armor and shield presence via `ArmorContributors()`. Computes AC using `ACFromArmor`. Special case: **Breggles** get +1 AC when unarmored or wearing light armor (Bulk 1) due to their tough fur.

### `ArmorContributors(items)`
Scans equipped items for armor and shield. Returns armor name, shield presence, and magic bonus. Only considers items that are equipped (not stowed, not on companions). Handles magic bonus prefix (e.g., "+1 Leather").

## Save Targets (`engine/class.go`)

The `SaveTargets` struct holds five saving throw target numbers:
- **Doom** (death/poison)
- **Ray** (wands/rays)
- **Hold** (paralysis/petrification)
- **Blast** (breath attacks)
- **Spell** (spells/staves)

`ClassSaveTargets(class, level)` extracts save targets for any class and level by parsing the last 5 columns of the class's advancement table. See [XP and Advancement](xp-and-advancement.md) for details.

## Conditional Save Bonuses (`engine/save_bonuses.go`)

`ConditionalSaveBonuses(kindred, class, level, moonSign)` aggregates bonuses from three sources:

### Kindred Save Bonuses
Scans `KindredTraits()` for save-relevant traits. Currently only **Mossling's "Resilience"** qualifies.

### Class Save Bonuses
Scans `ClassTraits()` for save-relevant traits:
- **Knight's "Strength of Will"** (level 3+)
- **Hunter's "Trophies"**

### Moon Sign Save Bonuses
If a character has a birthday (and thus a moon sign), any moon sign effect mentioning "saving throw" produces a save bonus. For compound effects (like Narrow Moon Waxing which has both a reaction bonus and a save penalty), `extractSavePart()` isolates the save-related clause.

Each bonus is a `SaveBonus{Source, Description}` struct.

## Magic Resistance (`engine/magic_resistance.go`)

`MagicResistance(kindred, wisdom)` returns:
- WIS modifier (from `Modifier()`)
- Plus kindred bonus: **+2 for Elves and Grimalkin**, 0 for all others

Case-insensitive kindred matching.
