package engine

import "strings"

// MagicResistance returns the total magic resistance modifier for a character
// based on wisdom and kindred bonuses.
func MagicResistance(kindred string, wisdom int) int {
	bonus := 0
	switch strings.ToLower(kindred) {
	case "elf", "grimalkin":
		bonus = 2
	}
	return Modifier(wisdom) + bonus
}
