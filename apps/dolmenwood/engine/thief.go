package engine

import (
	"maps"
	"strings"
)

type SkillTargets map[string]int

func ThiefBackstabDamage() string {
	return "3d4"
}

func ThiefBackstabBonus() int {
	return 4
}

func ThiefSkillTargets(level int) SkillTargets {
	rows := thiefSkillTargetsTable()
	if len(rows) == 0 {
		return nil
	}
	if level < 1 || level > len(rows) {
		return nil
	}
	targets := make(SkillTargets, len(rows[0]))
	maps.Copy(targets, rows[level-1])
	return targets
}

func thiefSkillTargetsTable() []SkillTargets {
	return []SkillTargets{
		{
			"Climb Wall":   4,
			"Decipher Doc.": 6,
			"Disarm Mech.":  6,
			"Legerdemain":  6,
			"Listen":        6,
			"Pick Lock":     5,
			"Search":        6,
			"Stealth":       5,
		},
		{
			"Climb Wall":   4,
			"Decipher Doc.": 6,
			"Disarm Mech.":  5,
			"Legerdemain":  6,
			"Listen":        6,
			"Pick Lock":     5,
			"Search":        5,
			"Stealth":       5,
		},
		{
			"Climb Wall":   4,
			"Decipher Doc.": 6,
			"Disarm Mech.":  5,
			"Legerdemain":  5,
			"Listen":        5,
			"Pick Lock":     5,
			"Search":        5,
			"Stealth":       5,
		},
		{
			"Climb Wall":   3,
			"Decipher Doc.": 5,
			"Disarm Mech.":  5,
			"Legerdemain":  5,
			"Listen":        5,
			"Pick Lock":     5,
			"Search":        5,
			"Stealth":       5,
		},
		{
			"Climb Wall":   3,
			"Decipher Doc.": 5,
			"Disarm Mech.":  5,
			"Legerdemain":  5,
			"Listen":        5,
			"Pick Lock":     4,
			"Search":        5,
			"Stealth":       4,
		},
		{
			"Climb Wall":   3,
			"Decipher Doc.": 5,
			"Disarm Mech.":  4,
			"Legerdemain":  5,
			"Listen":        5,
			"Pick Lock":     4,
			"Search":        4,
			"Stealth":       4,
		},
		{
			"Climb Wall":   3,
			"Decipher Doc.": 5,
			"Disarm Mech.":  4,
			"Legerdemain":  4,
			"Listen":        4,
			"Pick Lock":     4,
			"Search":        4,
			"Stealth":       4,
		},
		{
			"Climb Wall":   2,
			"Decipher Doc.": 4,
			"Disarm Mech.":  4,
			"Legerdemain":  4,
			"Listen":        4,
			"Pick Lock":     4,
			"Search":        4,
			"Stealth":       4,
		},
		{
			"Climb Wall":   2,
			"Decipher Doc.": 4,
			"Disarm Mech.":  4,
			"Legerdemain":  4,
			"Listen":        4,
			"Pick Lock":     3,
			"Search":        4,
			"Stealth":       3,
		},
		{
			"Climb Wall":   2,
			"Decipher Doc.": 4,
			"Disarm Mech.":  3,
			"Legerdemain":  4,
			"Listen":        4,
			"Pick Lock":     3,
			"Search":        3,
			"Stealth":       3,
		},
		{
			"Climb Wall":   2,
			"Decipher Doc.": 4,
			"Disarm Mech.":  3,
			"Legerdemain":  3,
			"Listen":        3,
			"Pick Lock":     3,
			"Search":        3,
			"Stealth":       3,
		},
		{
			"Climb Wall":   2,
			"Decipher Doc.": 3,
			"Disarm Mech.":  3,
			"Legerdemain":  3,
			"Listen":        3,
			"Pick Lock":     2,
			"Search":        3,
			"Stealth":       3,
		},
		{
			"Climb Wall":   2,
			"Decipher Doc.": 3,
			"Disarm Mech.":  3,
			"Legerdemain":  3,
			"Listen":        3,
			"Pick Lock":     2,
			"Search":        2,
			"Stealth":       2,
		},
		{
			"Climb Wall":   2,
			"Decipher Doc.": 3,
			"Disarm Mech.":  2,
			"Legerdemain":  3,
			"Listen":        2,
			"Pick Lock":     2,
			"Search":        2,
			"Stealth":       2,
		},
		{
			"Climb Wall":   2,
			"Decipher Doc.": 2,
			"Disarm Mech.":  2,
			"Legerdemain":  2,
			"Listen":        2,
			"Pick Lock":     2,
			"Search":        2,
			"Stealth":       2,
		},
	}
}

var thiefSkillNames = []string{
	"Climb Wall",
	"Decipher Doc.",
	"Disarm Mech.",
	"Legerdemain",
	"Listen",
	"Pick Lock",
	"Search",
	"Stealth",
}

func ThiefSkillNames() []string {
	return append([]string(nil), thiefSkillNames...)
}

func IsThiefClass(class string) bool {
	return strings.EqualFold(class, "thief")
}
