# Kindred and Class Traits

## Overview (`engine/traits.go`)

Traits represent special abilities granted by a character's kindred (race) and class. Each trait has a `Name` and `Description`. Some traits are level-dependent, appearing or scaling at specific levels.

## Kindred Traits

`KindredTraits(kindred, level)` returns traits for a given kindred. Six kindreds are supported:

### Human
- **Decisiveness**: Win ties on initiative.
- **Leadership**: +1 retainer loyalty.
- **Spirited**: +10% XP.

### Elf
- **Elf Skills**: Listen and Search target 5.
- **Glamours**: Know one randomly determined glamour.
- **Immortality**: Immune to non-magical disease; no natural death.
- **Magic Resistance**: +2 magic resistance.
- **Unearthly Beauty**: +2 CHA with mortals (max 18).
- **Vulnerable to Cold Iron**: Cold iron weapons deal +1 damage.

### Grimalkin
- **Armour and Weapons**: Must tailor armour; cannot wield Large weapons.
- **Defensive Bonus**: +2 AC vs Large creatures in melee.
- **Eating Giant Rodents**: Spend 1 Turn eating a giant rodent to heal 1 HP.
- **Glamours**: Know one randomly determined glamour.
- **Grimalkin Skills**: Listen skill target 5.
- **Immortality**: Immune to non-magical disease; no natural death.
- **Magic Resistance**: +2 magic resistance.
- **Shape-Shifting**: Shift between estray, chester, and wilder forms.
- **Vulnerable to Cold Iron**: Cold iron weapons deal +1 damage.

### Mossling
- **Armour and Weapons**: Must tailor armour; cannot wield Large weapons.
- **Knacks**: Know one mossling knack.
- **Mossling Skills**: Survival skill target 5 when foraging.
- **Resilience**: +4 saves vs fungal spores/poisons; +2 to other saves.
- **Symbiotic Flesh**: Gain a random symbiotic flesh trait each level.

### Woodgrue
- **Armour and Weapons**: Must tailor armour; cannot wield Large weapons.
- **Compulsive Jubilation**: Must partake in revelry or save to resist.
- **Defensive Bonus**: +2 AC vs Large creatures in melee.
- **Mad Revelry**: Play enchanted melodies once per day.
- **Moon Sight**: See in darkness up to 60'.
- **Musical Instruments**: Can use as melee weapon (1d4 damage).
- **Vulnerable to Cold Iron**: Cold iron weapons deal +1 damage.

### Breggle (Level-Dependent)
- **Horns**: Natural weapon, damage scales with level:
  - Levels 1-2: 1d4
  - Levels 3-4: 1d4+1
  - Levels 5-6: 1d6
  - Levels 7-9: 1d6+1
  - Level 10+: 1d6+2
- **Tough Fur**: +1 AC when unarmored or wearing light armor
- **Gaze** (level 4+ only): Mesmerizing gaze attack, uses scale with level:
  - Level 4-5: 1/day
  - Level 6-7: 2/day
  - Level 8-9: 3/day
  - Level 10+: 4/day

The Breggle horn damage is implemented in `engine/breggle.go:BreggleHornDamage(level)` and gaze uses in `BreggleGazeUses(level)`.

## Class Traits

`ClassTraits(class, level)` returns traits for a given class. Nine classes are supported. All class traits are returned as `[]Trait` — there is no class-specific struct. Level-gated traits are included only when the character has reached the required level.

### Fighter
- **Restrictions**: Must use martial weapons and armour.
- **Combat Talents**: Choose a combat talent at levels 2, 6, 10, and 14.

### Bard
- **Bard Skills**: Decipher Script, Monster Lore.
- **Counter Charm**: Neutralize enchanted music within 30'.
- **Enchantment**: Perform enchanted music (uses/day = level).

### Enchanter
- **Fairy Runes**: Read magical inscriptions.
- **Glamours**: Cast fairy glamours (count from advancement table).
- **Magic Items**: Use arcane magic items.
- **Restrictions**: Semi-martial weapons and armour.
- **Resistance to Divine Aid**: 2-in-6 chance of divine spells failing on enchanter.
- **Detect Magic**: Sense magical auras (skill target from advancement table).

### Knight (Level-Gated)
- **Chivalric Code**: Follow a knightly code of conduct.
- **Horsemanship**: Assess steeds and urge great speed (level 5+).
- **Mounted Combat**: +1 to attack when mounted.
- **Strength of Will**: +2 to saves vs fairy magic and fear.
- **Knighthood** (level 3+): Gain coat of arms and rights of hospitality.
- **Monster Slayer** (level 5+): +2 to attack and damage vs Large creatures.

### Cleric (Level-Gated)
- **Holy Magic**: Cast holy spells (from level 2+).
- **Turning the Undead**: Repel or destroy undead creatures.
- **Restrictions**: Semi-martial weapons and armour.
- **Languages**: Liturgic language.
- **Holy Order** (level 2+): Ordination into a religious order.
- **Holy Items**: Use holy magic items.
- **Detect Holy Magic Items**: Sense holy enchantments.

### Friar
- **Armour of Faith**: AC bonus by level (from advancement table).
- **Culinary Implements**: Improvise weapons from cooking tools (1d4).
- **Herbalism**: Identify and use medicinal herbs.
- **Holy Magic**: Cast holy spells (from level 1 if faith kept).
- **Poverty**: May not possess more than 20gp.
- **Restrictions**: Non-martial weapons only.
- **Languages**: Liturgic language.
- **Holy Items**: Use holy magic items.
- **Turning the Undead**: Repel or destroy undead creatures.
- **Detect Holy Magic Items**: Sense holy enchantments.

### Magician
- **Arcane Magic**: Cast arcane spells from spellbook.
- **Magic Items**: Use arcane magic items.
- **Magic Skills**: Detect Magic, Identify Item.
- **Restrictions**: Non-martial weapons only.

### Thief
- **Back-Stab**: +4 to attack, 3d4 damage when striking from behind.
- **Thief Skills**: Climb, Pick Locks, Disarm Traps (targets by level).
- **Restrictions**: Semi-martial weapons and armour.
- **Languages**: Thieves' Cant.

### Hunter
- **Animal Companion**: Bond with a wild animal at level 1.
- **Hunter Skills**: Alertness, Stalking, Tracking (targets by level).
- **Missile Attacks**: +1 to ranged attack rolls.
- **Trophies**: Bonuses from slain monster trophies (save bonus).
- **Restrictions**: Martial weapons and armour.

## XP Modifiers

`KindredXPModifier(kindred)` -- returns +10 for Humans, 0 for all others.

`TotalXPModifier(kindred, scores, primes)` -- combines `PrimeAbilityXPModifier` (from ability scores) and `KindredXPModifier`. Prime abilities per class are provided by `ClassPrimes(class)` in `engine/class.go`.
