package engine

import (
	"reflect"
	"testing"
)

func TestClassNames(t *testing.T) {
	want := []string{
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
	got := ClassNames()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ClassNames() = %v, want %v", got, want)
	}
}

func TestKindredNames(t *testing.T) {
	want := []string{
		"Human",
		"Elf",
		"Grimalkin",
		"Mossling",
		"Woodgrue",
		"Breggle",
	}
	got := KindredNames()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("KindredNames() = %v, want %v", got, want)
	}
}

func TestIsValidClass(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid", "Knight", true},
		{"valid lowercase", "thief", true},
		{"invalid", "Paladin", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidClass(tc.input); got != tc.want {
				t.Errorf("IsValidClass(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsValidKindred(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid", "Human", true},
		{"valid lowercase", "woodgrue", true},
		{"invalid", "Goblin", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidKindred(tc.input); got != tc.want {
				t.Errorf("IsValidKindred(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestFighterCombatTalents(t *testing.T) {
	cases := []struct {
		name  string
		level int
		want  int
	}{
		{"level 1", 1, 0},
		{"level 2", 2, 1},
		{"level 6", 6, 2},
		{"level 10", 10, 3},
		{"level 14", 14, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FighterCombatTalents(tc.level); got != tc.want {
				t.Errorf("FighterCombatTalents(%d) = %d, want %d", tc.level, got, tc.want)
			}
		})
	}
}

func TestIsEnchanterClass(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"enchanter", "Enchanter", true},
		{"enchanter lowercase", "enchanter", true},
		{"other class", "Thief", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsEnchanterClass(tc.input); got != tc.want {
				t.Errorf("IsEnchanterClass(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestEnchanterGlamours(t *testing.T) {
	cases := []struct {
		name  string
		level int
		want  int
	}{
		{"level 1", 1, 1},
		{"level 4", 4, 3},
		{"level 10", 10, 7},
		{"level 15", 15, 10},
		{"invalid level", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EnchanterGlamours(tc.level); got != tc.want {
				t.Errorf("EnchanterGlamours(%d) = %d, want %d", tc.level, got, tc.want)
			}
		})
	}
}

func TestClassPrimes(t *testing.T) {
	cases := []struct {
		class string
		want  []string
	}{
		{"Knight", []string{"str", "cha"}},
		{"Fighter", []string{"str"}},
		{"Hunter", []string{"con", "dex"}},
		{"Cleric", []string{"wis"}},
		{"Friar", []string{"int", "wis"}},
		{"Magician", []string{"int"}},
		{"Thief", []string{"dex"}},
		{"Bard", []string{"cha", "dex"}},
		{"Enchanter", []string{"cha", "int"}},
		{"Unknown", nil},
	}
	for _, tc := range cases {
		t.Run(tc.class, func(t *testing.T) {
			got := ClassPrimes(tc.class)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ClassPrimes(%q) = %v, want %v", tc.class, got, tc.want)
			}
		})
	}
}

func TestClassLevelForXP(t *testing.T) {
	cases := []struct {
		name  string
		class string
		xp    int
		want  int
	}{
		{"level 1 at zero", "Knight", 0, 1},
		{"still level 1", "Knight", 2249, 1},
		{"level 2 at 2250", "Knight", 2250, 2},
		{"level 3 at 4500", "Knight", 4500, 3},
		{"unknown class", "Paladin", 1000, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassLevelForXP(tc.class, tc.xp)
			if got != tc.want {
				t.Errorf("ClassLevelForXP(%q, %d) = %d, want %d", tc.class, tc.xp, got, tc.want)
			}
		})
	}
}

func TestClassXPForLevel(t *testing.T) {
	cases := []struct {
		name  string
		class string
		level int
		want  int
	}{
		{"level 1", "Knight", 1, 0},
		{"level 2", "Knight", 2, 2250},
		{"level 15", "Knight", 15, 1070000},
		{"unknown class", "Paladin", 1, 0},
		{"invalid level", "Knight", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassXPForLevel(tc.class, tc.level)
			if got != tc.want {
				t.Errorf("ClassXPForLevel(%q, %d) = %d, want %d", tc.class, tc.level, got, tc.want)
			}
		})
	}
}

func TestClassAttackBonus(t *testing.T) {
	cases := []struct {
		name  string
		class string
		level int
		want  int
	}{
		{"knight level 1", "Knight", 1, 1},
		{"knight level 3", "Knight", 3, 2},
		{"fighter level 10", "Fighter", 10, 7},
		{"unknown class", "Paladin", 1, 0},
		{"invalid level", "Knight", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassAttackBonus(tc.class, tc.level)
			if got != tc.want {
				t.Errorf("ClassAttackBonus(%q, %d) = %d, want %d", tc.class, tc.level, got, tc.want)
			}
		})
	}
}

func TestClassSaveTargets(t *testing.T) {
	cases := []struct {
		name  string
		class string
		level int
		want  SaveTargets
	}{
		{
			name:  "knight level 4",
			class: "Knight",
			level: 4,
			want:  SaveTargets{Doom: 10, Ray: 11, Hold: 10, Blast: 13, Spell: 13},
		},
		{
			name:  "enchanter level 1",
			class: "Enchanter",
			level: 1,
			want:  SaveTargets{Doom: 11, Ray: 12, Hold: 13, Blast: 16, Spell: 14},
		},
		{
			name:  "unknown class",
			class: "Paladin",
			level: 1,
			want:  SaveTargets{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassSaveTargets(tc.class, tc.level)
			if got != tc.want {
				t.Errorf("ClassSaveTargets(%q, %d) = %+v, want %+v", tc.class, tc.level, got, tc.want)
			}
		})
	}
}

func TestClassSpecificColumns(t *testing.T) {
	cases := []struct {
		name  string
		class string
		level int
		want  map[string]string
	}{
		{
			name:  "fighter talents",
			class: "Fighter",
			level: 2,
			want:  map[string]string{"Combat Talents": "1"},
		},
		{
			name:  "friar ac bonus",
			class: "Friar",
			level: 1,
			want:  map[string]string{"AC Bonus": "2"},
		},
		{
			name:  "bard none",
			class: "Bard",
			level: 1,
			want:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassSpecificColumns(tc.class, tc.level)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ClassSpecificColumns(%q, %d) = %v, want %v", tc.class, tc.level, got, tc.want)
			}
		})
	}
}
