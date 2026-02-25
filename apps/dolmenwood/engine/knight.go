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

// KnightLevelForXP returns the knight level for the given XP total.
func KnightLevelForXP(xp int) int {
	return ClassLevelForXP("Knight", xp)
}

// KnightAttackBonus returns the attack bonus for a knight at the given level.
func KnightAttackBonus(level int) int {
	return ClassAttackBonus("Knight", level)
}

// KnightSaveTargets returns the saving throw targets for a knight at the given level.
func KnightSaveTargets(level int) SaveTargets {
	return ClassSaveTargets("Knight", level)
}

// KnightTraits returns which class traits are unlocked at the given level.
func KnightTraits(level int) Traits {
	return Traits{
		MonsterSlayer: level >= 5,
		Knighthood:    level >= 3,
	}
}
