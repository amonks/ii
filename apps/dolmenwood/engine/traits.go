package engine

import "strings"

// Trait represents a class or kindred trait with a short description.
type Trait struct {
	Name        string
	Description string
}

// KindredTraits returns the traits granted by a kindred at the given level.
func KindredTraits(kindred string, level int) []Trait {
	switch strings.ToLower(kindred) {
	case "human":
		return []Trait{
			{Name: "Decisiveness", Description: "Win ties on initiative."},
			{Name: "Leadership", Description: "+1 retainer loyalty."},
			{Name: "Spirited", Description: "+10% XP."},
		}
	case "elf":
		return []Trait{
			{Name: "Elf Skills", Description: "Listen and Search target 5."},
			{Name: "Glamours", Description: "Know one randomly determined glamour."},
			{Name: "Immortality", Description: "Immune to non-magical disease; no natural death."},
			{Name: "Magic Resistance", Description: "+2 magic resistance."},
			{Name: "Unearthly Beauty", Description: "+2 CHA with mortals (max 18)."},
			{Name: "Vulnerable to Cold Iron", Description: "Cold iron weapons deal +1 damage."},
		}
	case "grimalkin":
		return []Trait{
			{Name: "Armour and Weapons", Description: "Must tailor armour; cannot wield Large weapons."},
			{Name: "Defensive Bonus", Description: "+2 AC vs Large creatures in melee."},
			{Name: "Eating Giant Rodents", Description: "Spend 1 Turn eating a giant rodent to heal 1 HP."},
			{Name: "Glamours", Description: "Know one randomly determined glamour."},
			{Name: "Grimalkin Skills", Description: "Listen skill target 5."},
			{Name: "Immortality", Description: "Immune to non-magical disease; no natural death."},
			{Name: "Magic Resistance", Description: "+2 magic resistance."},
			{Name: "Shape-Shifting", Description: "Shift between estray, chester, and wilder forms."},
			{Name: "Vulnerable to Cold Iron", Description: "Cold iron weapons deal +1 damage."},
		}
	case "mossling":
		return []Trait{
			{Name: "Armour and Weapons", Description: "Must tailor armour; cannot wield Large weapons."},
			{Name: "Knacks", Description: "Know one mossling knack."},
			{Name: "Mossling Skills", Description: "Survival skill target 5 when foraging."},
			{Name: "Resilience", Description: "+4 saves vs fungal spores/poisons; +2 to other saves."},
			{Name: "Symbiotic Flesh", Description: "Gain a random symbiotic flesh trait each level."},
		}
	case "woodgrue":
		return []Trait{
			{Name: "Armour and Weapons", Description: "Must tailor armour; cannot wield Large weapons."},
			{Name: "Compulsive Jubilation", Description: "Must partake in celebrations or save vs spell to resist."},
			{Name: "Defensive Bonus", Description: "+2 AC vs Large creatures in melee."},
			{Name: "Mad Revelry", Description: "Once per day, play an enchanted melody that affects nearby creatures."},
			{Name: "Moon Sight", Description: "See in darkness up to 60' without low-light penalties."},
			{Name: "Musical Instruments", Description: "May use a musical instrument as a melee weapon (1d4)."},
			{Name: "Starting Equipment", Description: "Begin play with a wind instrument."},
			{Name: "Vulnerable to Cold Iron", Description: "Cold iron weapons deal +1 damage."},
			{Name: "Woodgrue Skills", Description: "Listen skill target 5."},
		}
	case "breggle":
		traits := []Trait{
			{Name: "Fur", Description: "+1 AC when unarmoured or in light armour."},
			{Name: "Horns", Description: "Melee horn attack; damage scales with level."},
		}
		if level >= 4 {
			traits = append(traits, Trait{Name: "Gaze", Description: "Level 4: charm humans or shorthorns once per day (save vs spell)."})
		}
		return traits
	default:
		return nil
	}
}

// ClassTraits returns the traits granted by a class at the given level.
func ClassTraits(class string, level int) []Trait {
	switch strings.ToLower(class) {
	case "fighter":
		return []Trait{
			{Name: "Combat Talents", Description: "Select combat talents at levels 2, 6, 10, and 14."},
		}
	case "bard":
		return []Trait{
			{Name: "Bard Skills", Description: "Listen target 5; gains Decipher Document, Legerdemain, and Monster Lore skills."},
			{Name: "Counter Charm", Description: "While performing, allies within 30' are immune to song magic and gain +2 saves vs fairy magic once per Turn."},
			{Name: "Enchantment", Description: "Fascinate subjects within 30'; uses per day equal to level; expands to animals, demi-fey, and fairies with level."},
		}
	case "enchanter":
		return []Trait{
			{Name: "Restrictions", Description: "Typically fairies/demi-fey; mortal enchanters are rare."},
			{Name: "Enchanter Skills", Description: "Detect magic skill; see enchanter skill targets."},
			{Name: "Fairy Runes", Description: "Level 1: know one randomly selected lesser rune; chance for more each level."},
			{Name: "Glamours", Description: "Glamours known by level; kindred glamours are additional."},
			{Name: "Magic Items", Description: "May use arcane spell-caster items (wands, scrolls, etc.)."},
			{Name: "Resistance to Divine Aid", Description: "2-in-6 chance beneficial holy spells have no effect."},
		}
	case "knight":
		traits := []Trait{
			{Name: "Chivalric Code", Description: "Uphold the code of chivalry."},
			{Name: "Horsemanship", Description: "Assess steeds; from level 5, urge speed once per day."},
			{Name: "Mounted Combat", Description: "+1 attack when mounted."},
			{Name: "Strength of Will", Description: "+2 saves vs fairy magic and fear."},
		}
		if level >= 3 {
			traits = append(traits, Trait{Name: "Knighthood", Description: "Level 3: gain knighthood and hospitality."})
		}
		if level >= 5 {
			traits = append(traits, Trait{Name: "Monster Slayer", Description: "+2 attack/damage vs Large creatures."})
		}
		return traits
	case "cleric":
		return []Trait{
			{Name: "Restrictions", Description: "Lawful or Neutral; mortals only; holy magic armaments only."},
			{Name: "Cleric Tenets", Description: "Evangelism, hierarchy, monotheism, sanctity of life."},
			{Name: "Detect Holy Magic Items", Description: "Identify holy enchantments by touch with 1 Turn of focus."},
			{Name: "Holy Magic", Description: "Pray for holy spells; must carry a holy symbol."},
			{Name: "Holy Order", Description: "Level 2: choose a holy order and gain its power."},
			{Name: "Languages", Description: "Speaks Liturgic in addition to native languages."},
			{Name: "Turning the Undead", Description: "May drive off undead by presenting a holy symbol once per turn."},
		}
	case "friar":
		return []Trait{
			{Name: "Friar Tenets", Description: "Sanctity of life, monotheism, spiritual insight, and mentorship."},
			{Name: "Armour of Faith", Description: "Divine blessing grants an AC bonus by level."},
			{Name: "Culinary Implements", Description: "Can use frying pans and similar implements as melee weapons (1d4)."},
			{Name: "Friar Skills", Description: "Survival skill target 5 when foraging."},
			{Name: "Herbalism", Description: "One dose of medicinal herb heals two subjects."},
			{Name: "Holy Magic", Description: "Pray for holy spells; must carry a holy symbol."},
			{Name: "Languages", Description: "Speaks Liturgic in addition to native languages."},
			{Name: "Poverty", Description: "Limited possessions; excess wealth donated to worthy causes."},
			{Name: "Turning the Undead", Description: "May drive off undead by presenting a holy symbol once per turn."},
		}
	default:
		return nil
	}
}

// KindredXPModifier returns the XP modifier for a kindred.
func KindredXPModifier(kindred string) int {
	if strings.EqualFold(kindred, "human") {
		return humanXPBonus
	}
	return 0
}

// TotalXPModifier returns the total XP modifier for a character's kindred and primes.
func TotalXPModifier(kindred string, scores map[string]int, primes []string) int {
	return PrimeAbilityXPModifier(scores, primes) + KindredXPModifier(kindred)
}
