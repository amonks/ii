package engine

// ApplyXPModifiers applies a percentage modifier to base XP.
// modPercent is an integer percentage, e.g. 15 means +15%.
func ApplyXPModifiers(base, modPercent int) int {
	return base + (base * modPercent / 100)
}

// DetectLevelUp checks if the current XP qualifies for a higher level.
// Returns the new level and whether a level-up occurred.
func DetectLevelUp(class string, currentLevel, xp int) (int, bool) {
	newLevel := ClassLevelForXP(class, xp)
	if newLevel > currentLevel {
		return newLevel, true
	}
	return currentLevel, false
}

// XPToNextLevel returns how much XP is needed to reach the next level.
func XPToNextLevel(class string, currentLevel, currentXP int) int {
	nextLevel := currentLevel + 1
	if ClassXPForLevel(class, nextLevel) == 0 {
		return 0
	}
	nextLevelXP := ClassXPForLevel(class, nextLevel)
	remaining := nextLevelXP - currentXP
	if remaining < 0 {
		return 0
	}
	return remaining
}
