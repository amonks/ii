package engine

import "testing"

func TestKindredTraitsHuman(t *testing.T) {
	traits := KindredTraits("Human", 1)
	if len(traits) != 3 {
		t.Fatalf("expected 3 traits, got %d", len(traits))
	}
	if traits[0].Name != "Decisiveness" {
		t.Errorf("first trait = %q, want Decisiveness", traits[0].Name)
	}
	if traits[2].Name != "Spirited" {
		t.Errorf("third trait = %q, want Spirited", traits[2].Name)
	}
}

func TestKindredTraitsElf(t *testing.T) {
	traits := KindredTraits("Elf", 1)
	if len(traits) != 6 {
		t.Fatalf("expected 6 traits, got %d", len(traits))
	}
	if traits[0].Name != "Elf Skills" {
		t.Errorf("first trait = %q, want Elf Skills", traits[0].Name)
	}
	if traits[1].Name != "Glamours" {
		t.Errorf("second trait = %q, want Glamours", traits[1].Name)
	}
	if traits[5].Name != "Vulnerable to Cold Iron" {
		t.Errorf("sixth trait = %q, want Vulnerable to Cold Iron", traits[5].Name)
	}
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
	}
}

func TestClassTraitsKnightLevel5(t *testing.T) {
	traits := ClassTraits("Knight", 5)
	if len(traits) != 6 {
		t.Fatalf("expected 6 traits, got %d", len(traits))
	}
	if traits[0].Name != "Chivalric Code" {
		t.Errorf("first trait = %q, want Chivalric Code", traits[0].Name)
	}
	if traits[1].Name != "Horsemanship" {
		t.Errorf("second trait = %q, want Horsemanship", traits[1].Name)
	}
	if traits[1].Description == "" {
		t.Error("expected description for Horsemanship")
	}
	if traits[4].Name != "Knighthood" {
		t.Errorf("fifth trait = %q, want Knighthood", traits[4].Name)
	}
	if traits[5].Name != "Monster Slayer" {
		t.Errorf("sixth trait = %q, want Monster Slayer", traits[5].Name)
	}
	if traits[4].Description == "" || traits[5].Description == "" {
		t.Error("expected descriptions for level-gated traits")
	}
	foundMonster := false
	foundKnighthood := false
	for _, trait := range traits {
		if trait.Name == "Monster Slayer" {
			foundMonster = true
		}
		if trait.Name == "Knighthood" {
			foundKnighthood = true
		}
	}
	if !foundMonster {
		t.Error("expected Monster Slayer trait")
	}
	if !foundKnighthood {
		t.Error("expected Knighthood trait")
	}
}

func TestTotalXPModifier(t *testing.T) {
	scores := map[string]int{"str": 15}
	primes := []string{"str"}
	got := TotalXPModifier("Human", scores, primes)
	if got != 15 {
		t.Errorf("TotalXPModifier = %d, want 15", got)
	}
}
