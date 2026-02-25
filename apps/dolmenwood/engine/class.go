package engine

import (
	"strconv"
	"strings"
)

var classNames = []string{
	"Bard",
	"Cleric",
	"Enchanter",
	"Fighter",
	"Friar",
	"Hunter",
	"Knight",
	"Magician",
	"Thief",
}

var kindredNames = []string{
	"Human",
	"Elf",
	"Grimalkin",
	"Mossling",
	"Woodgrue",
	"Breggle",
}

var classPrimes = map[string][]string{
	"bard":      {"cha", "dex"},
	"cleric":    {"wis"},
	"enchanter": {"cha", "int"},
	"fighter":   {"str"},
	"friar":     {"int", "wis"},
	"hunter":    {"con", "dex"},
	"knight":    {"str", "cha"},
	"magician":  {"int"},
	"thief":     {"dex"},
}

var classNameSet = makeNameSet(classNames)
var kindredNameSet = makeNameSet(kindredNames)

// ClassNames returns the list of class names in display order.
func ClassNames() []string {
	return append([]string(nil), classNames...)
}

// KindredNames returns the list of kindred names in display order.
func KindredNames() []string {
	return append([]string(nil), kindredNames...)
}

// IsValidClass reports whether the class is supported (case-insensitive).
func IsValidClass(class string) bool {
	_, ok := classNameSet[strings.ToLower(class)]
	return ok
}

// IsValidKindred reports whether the kindred is supported (case-insensitive).
func IsValidKindred(kindred string) bool {
	_, ok := kindredNameSet[strings.ToLower(kindred)]
	return ok
}

// ClassPrimes returns the prime ability score names for a class.
func ClassPrimes(class string) []string {
	primes, ok := classPrimes[strings.ToLower(class)]
	if !ok {
		return nil
	}
	return append([]string(nil), primes...)
}

func makeNameSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, name := range values {
		set[strings.ToLower(name)] = struct{}{}
	}
	return set
}

// ClassAttackBonus returns the attack bonus for the given class and level.
func ClassAttackBonus(class string, level int) int {
	table, ok := AdvancementTableForClass(class)
	if !ok {
		return 0
	}
	row, ok := classRowForLevel(table, level)
	if !ok || len(row) < 4 {
		return 0
	}
	bonus, err := parseSignedInt(row[3])
	if err != nil {
		return 0
	}
	return bonus
}

// ClassSaveTargets returns the saving throw targets for a class at the given level.
func ClassSaveTargets(class string, level int) SaveTargets {
	table, ok := AdvancementTableForClass(class)
	if !ok {
		return SaveTargets{}
	}
	row, ok := classRowForLevel(table, level)
	if !ok || len(row) < 5 {
		return SaveTargets{}
	}
	start := len(row) - 5
	saves := SaveTargets{}
	values := []*int{&saves.Doom, &saves.Ray, &saves.Hold, &saves.Blast, &saves.Spell}
	for i := range 5 {
		value, err := strconv.Atoi(row[start+i])
		if err != nil {
			return SaveTargets{}
		}
		*values[i] = value
	}
	return saves
}

// ClassSpecificColumns returns class-specific values between Attack and saves.
func ClassSpecificColumns(class string, level int) map[string]string {
	table, ok := AdvancementTableForClass(class)
	if !ok {
		return nil
	}
	row, ok := classRowForLevel(table, level)
	if !ok {
		return nil
	}
	start := 4
	end := len(row) - 5
	if end <= start || len(table.Headers) < end {
		return nil
	}
	values := make(map[string]string, end-start)
	for i := start; i < end; i++ {
		values[table.Headers[i]] = row[i]
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

// ClassLevelForXP returns the level for the given XP in the class advancement table.
func ClassLevelForXP(class string, xp int) int {
	table, ok := AdvancementTableForClass(class)
	if !ok {
		return 0
	}
	level := 0
	for _, row := range table.Rows {
		if len(row) < 2 {
			continue
		}
		threshold, err := parseXPValue(row[1])
		if err != nil {
			continue
		}
		if xp >= threshold {
			lvl, err := strconv.Atoi(row[0])
			if err != nil {
				continue
			}
			level = lvl
		}
	}
	return level
}

// ClassXPForLevel returns the XP threshold for the given class level.
func ClassXPForLevel(class string, level int) int {
	table, ok := AdvancementTableForClass(class)
	if !ok {
		return 0
	}
	for _, row := range table.Rows {
		if len(row) < 2 {
			continue
		}
		lvl, err := strconv.Atoi(row[0])
		if err != nil {
			continue
		}
		if lvl == level {
			threshold, err := parseXPValue(row[1])
			if err != nil {
				return 0
			}
			return threshold
		}
	}
	return 0
}

func classRowForLevel(table AdvancementTable, level int) ([]string, bool) {
	for _, row := range table.Rows {
		if len(row) == 0 {
			continue
		}
		lvl, err := strconv.Atoi(row[0])
		if err != nil {
			continue
		}
		if lvl == level {
			return row, true
		}
	}
	return nil, false
}

func parseSignedInt(value string) (int, error) {
	cleaned := strings.TrimPrefix(value, "+")
	return strconv.Atoi(cleaned)
}

func parseXPValue(value string) (int, error) {
	cleaned := strings.ReplaceAll(value, ",", "")
	return strconv.Atoi(cleaned)
}
