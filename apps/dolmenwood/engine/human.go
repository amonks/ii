package engine

const humanXPBonus = 10 // Humans get +10% XP

// HumanTotalXPModifier returns the total XP modifier for a human character,
// combining the prime ability XP modifier with the human +10% bonus.
func HumanTotalXPModifier(scores map[string]int, primes []string) int {
	return PrimeAbilityXPModifier(scores, primes) + humanXPBonus
}
