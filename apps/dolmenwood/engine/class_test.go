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
