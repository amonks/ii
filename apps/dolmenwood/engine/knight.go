package engine

type SaveTargets struct {
	Doom  int
	Ray   int
	Hold  int
	Blast int
	Spell int
}

type Traits struct {
	MonsterSlayer bool
	Knighthood    bool
}

type knightLevel struct {
	XPRequired  int
	AttackBonus int
	Saves       SaveTargets
}

// Knight advancement table (levels 1-15).
// Index 0 = level 1, index 14 = level 15.
var knightTable = []knightLevel{
	{0, 1, SaveTargets{12, 13, 12, 15, 15}},         // Level 1
	{2250, 1, SaveTargets{12, 13, 12, 15, 15}},       // Level 2
	{4500, 2, SaveTargets{12, 13, 12, 15, 15}},       // Level 3
	{9000, 3, SaveTargets{10, 11, 10, 13, 13}},       // Level 4
	{18000, 3, SaveTargets{10, 11, 10, 13, 13}},      // Level 5
	{36000, 4, SaveTargets{10, 11, 10, 13, 13}},      // Level 6
	{70000, 5, SaveTargets{8, 9, 8, 10, 11}},         // Level 7
	{140000, 5, SaveTargets{8, 9, 8, 10, 11}},        // Level 8
	{270000, 6, SaveTargets{8, 9, 8, 10, 11}},        // Level 9
	{400000, 7, SaveTargets{6, 7, 6, 8, 9}},          // Level 10
	{530000, 7, SaveTargets{6, 7, 6, 8, 9}},          // Level 11
	{660000, 8, SaveTargets{6, 7, 6, 8, 9}},          // Level 12
	{790000, 9, SaveTargets{4, 5, 4, 6, 7}},          // Level 13
	{920000, 9, SaveTargets{4, 5, 4, 6, 7}},          // Level 14
	{1050000, 10, SaveTargets{4, 5, 4, 6, 7}},        // Level 15
}

// KnightLevelForXP returns the knight level for the given XP total.
func KnightLevelForXP(xp int) int {
	level := 1
	for i, entry := range knightTable {
		if xp >= entry.XPRequired {
			level = i + 1
		}
	}
	return level
}

// KnightAttackBonus returns the attack bonus for a knight at the given level.
func KnightAttackBonus(level int) int {
	if level < 1 || level > len(knightTable) {
		return 0
	}
	return knightTable[level-1].AttackBonus
}

// KnightSaveTargets returns the saving throw targets for a knight at the given level.
func KnightSaveTargets(level int) SaveTargets {
	if level < 1 || level > len(knightTable) {
		return SaveTargets{}
	}
	return knightTable[level-1].Saves
}

// KnightTraits returns which class traits are unlocked at the given level.
func KnightTraits(level int) Traits {
	return Traits{
		MonsterSlayer: level >= 5,
		Knighthood:    level >= 3,
	}
}
