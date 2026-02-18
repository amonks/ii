package engine

// ApplyXPModifiers applies a percentage modifier to base XP.
// modPercent is an integer percentage, e.g. 15 means +15%.
func ApplyXPModifiers(base, modPercent int) int {
	return base + (base * modPercent / 100)
}

// DetectLevelUp checks if the current XP qualifies for a higher level.
// Returns the new level and whether a level-up occurred.
func DetectLevelUp(currentLevel, xp int) (int, bool) {
	newLevel := KnightLevelForXP(xp)
	if newLevel > currentLevel {
		return newLevel, true
	}
	return currentLevel, false
}

// XPToNextLevel returns how much XP is needed to reach the next level.
func XPToNextLevel(currentLevel, currentXP int) int {
	if currentLevel >= len(knightTable) {
		return 0
	}
	nextLevelXP := knightTable[currentLevel].XPRequired
	remaining := nextLevelXP - currentXP
	if remaining < 0 {
		return 0
	}
	return remaining
}
