package engine

import "maps"

func BardSkillTargets(level int) SkillTargets {
	rows := bardSkillTargetsTable()
	if level < 1 || level > len(rows) {
		return nil
	}
	targets := make(SkillTargets, len(rows[0]))
	maps.Copy(targets, rows[level-1])
	return targets
}

func BardSkillNames() []string {
	return append([]string(nil), bardSkillNames...)
}

func bardSkillTargetsTable() []SkillTargets {
	return []SkillTargets{
		{"Decipher Doc.": 6, "Legerdemain": 6, "Listen": 5, "Monster Lore": 5},
		{"Decipher Doc.": 5, "Legerdemain": 6, "Listen": 5, "Monster Lore": 5},
		{"Decipher Doc.": 5, "Legerdemain": 6, "Listen": 5, "Monster Lore": 4},
		{"Decipher Doc.": 5, "Legerdemain": 5, "Listen": 5, "Monster Lore": 4},
		{"Decipher Doc.": 5, "Legerdemain": 5, "Listen": 4, "Monster Lore": 4},
		{"Decipher Doc.": 4, "Legerdemain": 5, "Listen": 4, "Monster Lore": 4},
		{"Decipher Doc.": 4, "Legerdemain": 5, "Listen": 4, "Monster Lore": 3},
		{"Decipher Doc.": 4, "Legerdemain": 4, "Listen": 4, "Monster Lore": 3},
		{"Decipher Doc.": 4, "Legerdemain": 4, "Listen": 3, "Monster Lore": 3},
		{"Decipher Doc.": 3, "Legerdemain": 4, "Listen": 3, "Monster Lore": 3},
		{"Decipher Doc.": 3, "Legerdemain": 3, "Listen": 3, "Monster Lore": 3},
		{"Decipher Doc.": 3, "Legerdemain": 3, "Listen": 3, "Monster Lore": 2},
		{"Decipher Doc.": 2, "Legerdemain": 3, "Listen": 3, "Monster Lore": 2},
		{"Decipher Doc.": 2, "Legerdemain": 3, "Listen": 2, "Monster Lore": 2},
		{"Decipher Doc.": 2, "Legerdemain": 2, "Listen": 2, "Monster Lore": 2},
	}
}

var bardSkillNames = []string{"Decipher Doc.", "Legerdemain", "Listen", "Monster Lore"}

func HunterSkillTargets(level int) SkillTargets {
	rows := hunterSkillTargetsTable()
	if level < 1 || level > len(rows) {
		return nil
	}
	targets := make(SkillTargets, len(rows[0]))
	maps.Copy(targets, rows[level-1])
	return targets
}

func HunterSkillNames() []string {
	return append([]string(nil), hunterSkillNames...)
}

func hunterSkillTargetsTable() []SkillTargets {
	return []SkillTargets{
		{"Alertness": 6, "Stalking": 6, "Survival": 5, "Tracking": 5},
		{"Alertness": 6, "Stalking": 6, "Survival": 4, "Tracking": 5},
		{"Alertness": 6, "Stalking": 6, "Survival": 4, "Tracking": 4},
		{"Alertness": 6, "Stalking": 5, "Survival": 4, "Tracking": 4},
		{"Alertness": 5, "Stalking": 5, "Survival": 4, "Tracking": 4},
		{"Alertness": 5, "Stalking": 5, "Survival": 3, "Tracking": 4},
		{"Alertness": 5, "Stalking": 5, "Survival": 3, "Tracking": 3},
		{"Alertness": 5, "Stalking": 4, "Survival": 3, "Tracking": 3},
		{"Alertness": 4, "Stalking": 4, "Survival": 3, "Tracking": 3},
		{"Alertness": 4, "Stalking": 3, "Survival": 3, "Tracking": 3},
		{"Alertness": 4, "Stalking": 3, "Survival": 2, "Tracking": 3},
		{"Alertness": 4, "Stalking": 3, "Survival": 2, "Tracking": 2},
		{"Alertness": 3, "Stalking": 3, "Survival": 2, "Tracking": 2},
		{"Alertness": 3, "Stalking": 2, "Survival": 2, "Tracking": 2},
		{"Alertness": 2, "Stalking": 2, "Survival": 2, "Tracking": 2},
	}
}

var hunterSkillNames = []string{"Alertness", "Stalking", "Survival", "Tracking"}
