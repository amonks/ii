package engine

// Modifier returns the B/X ability score modifier for a given score.
// 3: -3, 4-5: -2, 6-8: -1, 9-12: 0, 13-15: +1, 16-17: +2, 18: +3
func Modifier(score int) int {
	switch {
	case score <= 3:
		return -3
	case score <= 5:
		return -2
	case score <= 8:
		return -1
	case score <= 12:
		return 0
	case score <= 15:
		return 1
	case score <= 17:
		return 2
	default:
		return 3
	}
}

// PrimeAbilityXPModifier returns the XP modifier percentage based on
// prime ability scores. With multiple primes, it takes the lowest modifier.
// 3-5: -20%, 6-8: -10%, 9-12: 0%, 13-15: +5%, 16-18: +10%
func PrimeAbilityXPModifier(scores map[string]int, primes []string) int {
	lowest := 100 // sentinel
	for _, p := range primes {
		mod := primeXPMod(scores[p])
		if mod < lowest {
			lowest = mod
		}
	}
	if lowest == 100 {
		return 0
	}
	return lowest
}

func primeXPMod(score int) int {
	switch {
	case score <= 5:
		return -20
	case score <= 8:
		return -10
	case score <= 12:
		return 0
	case score <= 15:
		return 5
	default:
		return 10
	}
}

// ACFromArmor computes armor class from base armor AC, DEX score, and shield.
func ACFromArmor(baseAC, dexScore int, hasShield bool) int {
	ac := baseAC + Modifier(dexScore)
	if hasShield {
		ac++
	}
	return ac
}
