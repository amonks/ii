package engine

import "testing"

func TestMoonSignFromBirthday(t *testing.T) {
	sign, ok := MoonSignFromBirthday("Grimvold", 18)
	if !ok {
		t.Fatal("expected moon sign for Grimvold 18")
	}
	if sign.Moon != "Grinning moon" {
		t.Errorf("Moon = %q, want %q", sign.Moon, "Grinning moon")
	}
	if sign.Phase != "Full" {
		t.Errorf("Phase = %q, want %q", sign.Phase, "Full")
	}
	if sign.Effect != "+1 bonus to saving throws against the powers of undead monsters." {
		t.Errorf("Effect = %q, want full moon effect", sign.Effect)
	}

	waning, ok := MoonSignFromBirthday("Lymewald", 2)
	if !ok {
		t.Fatal("expected moon sign for Lymewald 2")
	}
	if waning.Moon != "Grinning moon" {
		t.Errorf("Moon = %q, want %q", waning.Moon, "Grinning moon")
	}
	if waning.Phase != "Waning" {
		t.Errorf("Phase = %q, want %q", waning.Phase, "Waning")
	}
}

func TestMoonSignFromBirthdayRejectsInvalidDate(t *testing.T) {
	if _, ok := MoonSignFromBirthday("Lymewald", 29); ok {
		t.Fatal("expected invalid date for Lymewald 29")
	}
}
