package engine

import "strings"

type SpellSlots struct {
	Level1 int
	Level2 int
	Level3 int
	Level4 int
	Level5 int
	Level6 int
}

var spellSlotsByClass = map[string][]SpellSlots{
	"cleric": {
		{},
		{Level1: 1},
		{Level1: 2},
		{Level1: 2, Level2: 1},
		{Level1: 2, Level2: 2},
		{Level1: 2, Level2: 2, Level3: 1},
		{Level1: 3, Level2: 2, Level3: 2},
		{Level1: 3, Level2: 2, Level3: 2},
		{Level1: 3, Level2: 3, Level3: 2, Level4: 1},
		{Level1: 3, Level2: 3, Level3: 2, Level4: 2},
		{Level1: 4, Level2: 3, Level3: 3, Level4: 2},
		{Level1: 4, Level2: 3, Level3: 3, Level4: 2, Level5: 1},
		{Level1: 4, Level2: 4, Level3: 3, Level4: 2, Level5: 2},
		{Level1: 4, Level2: 4, Level3: 3, Level4: 3, Level5: 2},
		{Level1: 5, Level2: 4, Level3: 4, Level4: 3, Level5: 2},
	},
	"friar": {
		{Level1: 1},
		{Level1: 2},
		{Level1: 2, Level2: 1},
		{Level1: 2, Level2: 2},
		{Level1: 3, Level2: 2, Level3: 1},
		{Level1: 3, Level2: 2, Level3: 2},
		{Level1: 3, Level2: 3, Level3: 2, Level4: 1},
		{Level1: 4, Level2: 3, Level3: 2, Level4: 2},
		{Level1: 4, Level2: 3, Level3: 3, Level4: 2, Level5: 1},
		{Level1: 4, Level2: 4, Level3: 3, Level4: 2, Level5: 2},
		{Level1: 5, Level2: 4, Level3: 3, Level4: 3, Level5: 2},
		{Level1: 5, Level2: 4, Level3: 4, Level4: 3, Level5: 2},
		{Level1: 5, Level2: 5, Level3: 4, Level4: 3, Level5: 3},
		{Level1: 6, Level2: 5, Level3: 4, Level4: 4, Level5: 3},
		{Level1: 6, Level2: 5, Level3: 5, Level4: 4, Level5: 3},
	},
	"magician": {
		{Level1: 1},
		{Level1: 2},
		{Level1: 2, Level2: 1},
		{Level1: 2, Level2: 2},
		{Level1: 2, Level2: 2, Level3: 1},
		{Level1: 3, Level2: 2, Level3: 2},
		{Level1: 3, Level2: 2, Level3: 2, Level4: 1},
		{Level1: 3, Level2: 3, Level3: 2, Level4: 2},
		{Level1: 3, Level2: 3, Level3: 2, Level4: 2, Level5: 1},
		{Level1: 4, Level2: 3, Level3: 3, Level4: 2, Level5: 2},
		{Level1: 4, Level2: 3, Level3: 3, Level4: 2, Level5: 2, Level6: 1},
		{Level1: 4, Level2: 4, Level3: 3, Level4: 3, Level5: 2, Level6: 2},
		{Level1: 4, Level2: 4, Level3: 3, Level4: 3, Level5: 3, Level6: 2},
		{Level1: 5, Level2: 4, Level3: 4, Level4: 3, Level5: 3, Level6: 2},
		{Level1: 5, Level2: 4, Level3: 4, Level4: 3, Level5: 3, Level6: 3},
	},
}

// ClassSpellSlots returns the spell slot counts for a class at a given level.
// Returns nil for non-spellcasting classes or invalid levels.
func ClassSpellSlots(class string, level int) *SpellSlots {
	if level < 1 {
		return nil
	}
	rows, ok := spellSlotsByClass[strings.ToLower(class)]
	if !ok {
		return nil
	}
	if level > len(rows) {
		return nil
	}
	slots := rows[level-1]
	return &slots
}
