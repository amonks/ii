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

func TestKindredTraitsBreggleLevel1(t *testing.T) {
	traits := KindredTraits("Breggle", 1)
	if len(traits) != 2 {
		t.Fatalf("expected 2 traits, got %d", len(traits))
	}
	if traits[0].Name != "Fur" {
		t.Errorf("first trait = %q, want Fur", traits[0].Name)
	}
	if traits[1].Name != "Horns" {
		t.Errorf("second trait = %q, want Horns", traits[1].Name)
	}
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
	}
}

func TestKindredTraitsBreggleLevel4(t *testing.T) {
	traits := KindredTraits("Breggle", 4)
	if len(traits) != 3 {
		t.Fatalf("expected 3 traits, got %d", len(traits))
	}
	if traits[2].Name != "Gaze" {
		t.Errorf("third trait = %q, want Gaze", traits[2].Name)
	}
	if traits[2].Description == "" {
		t.Error("expected description for Gaze")
	}
}

func TestKindredTraitsGrimalkin(t *testing.T) {
	traits := KindredTraits("Grimalkin", 1)
	if len(traits) != 9 {
		t.Fatalf("expected 9 traits, got %d", len(traits))
	}
	if traits[0].Name != "Armour and Weapons" {
		t.Errorf("first trait = %q, want Armour and Weapons", traits[0].Name)
	}
	if traits[8].Name != "Vulnerable to Cold Iron" {
		t.Errorf("ninth trait = %q, want Vulnerable to Cold Iron", traits[8].Name)
	}
	foundShapeShifting := false
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
		if trait.Name == "Shape-Shifting" {
			foundShapeShifting = true
		}
	}
	if !foundShapeShifting {
		t.Error("expected Shape-Shifting trait")
	}
}

func TestKindredTraitsMossling(t *testing.T) {
	traits := KindredTraits("Mossling", 1)
	if len(traits) != 5 {
		t.Fatalf("expected 5 traits, got %d", len(traits))
	}
	if traits[0].Name != "Armour and Weapons" {
		t.Errorf("first trait = %q, want Armour and Weapons", traits[0].Name)
	}
	if traits[1].Name != "Knacks" {
		t.Errorf("second trait = %q, want Knacks", traits[1].Name)
	}
	if traits[4].Name != "Symbiotic Flesh" {
		t.Errorf("fifth trait = %q, want Symbiotic Flesh", traits[4].Name)
	}
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
	}
}

func TestKindredTraitsWoodgrue(t *testing.T) {
	traits := KindredTraits("Woodgrue", 1)
	if len(traits) != 9 {
		t.Fatalf("expected 9 traits, got %d", len(traits))
	}
	if traits[0].Name != "Armour and Weapons" {
		t.Errorf("first trait = %q, want Armour and Weapons", traits[0].Name)
	}
	if traits[8].Name != "Woodgrue Skills" {
		t.Errorf("ninth trait = %q, want Woodgrue Skills", traits[8].Name)
	}
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
	}
}

func TestClassTraitsKnightLevel5(t *testing.T) {
	traits := ClassTraits("Knight", 5)
	if len(traits) != 7 {
		t.Fatalf("expected 7 traits, got %d", len(traits))
	}
	if traits[0].Name != "Restrictions" {
		t.Errorf("first trait = %q, want Restrictions", traits[0].Name)
	}
	if traits[1].Name != "Chivalric Code" {
		t.Errorf("second trait = %q, want Chivalric Code", traits[1].Name)
	}
	if traits[2].Name != "Horsemanship" {
		t.Errorf("third trait = %q, want Horsemanship", traits[2].Name)
	}
	if traits[2].Description == "" {
		t.Error("expected description for Horsemanship")
	}
	if traits[5].Name != "Knighthood" {
		t.Errorf("sixth trait = %q, want Knighthood", traits[5].Name)
	}
	if traits[6].Name != "Monster Slayer" {
		t.Errorf("seventh trait = %q, want Monster Slayer", traits[6].Name)
	}
	if traits[5].Description == "" || traits[6].Description == "" {
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

func TestClassTraitsFriar(t *testing.T) {
	traits := ClassTraits("Friar", 1)
	if len(traits) != 9 {
		t.Fatalf("expected 9 traits, got %d", len(traits))
	}
	if traits[0].Name != "Friar Tenets" {
		t.Errorf("first trait = %q, want Friar Tenets", traits[0].Name)
	}
	if traits[1].Name != "Armour of Faith" {
		t.Errorf("second trait = %q, want Armour of Faith", traits[1].Name)
	}
	if traits[8].Name != "Turning the Undead" {
		t.Errorf("ninth trait = %q, want Turning the Undead", traits[8].Name)
	}
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
	}
}

func TestClassTraitsCleric(t *testing.T) {
	traits := ClassTraits("Cleric", 1)
	if len(traits) != 7 {
		t.Fatalf("expected 7 traits, got %d", len(traits))
	}
	if traits[0].Name != "Restrictions" {
		t.Errorf("first trait = %q, want Restrictions", traits[0].Name)
	}
	if traits[1].Name != "Cleric Tenets" {
		t.Errorf("second trait = %q, want Cleric Tenets", traits[1].Name)
	}
	if traits[6].Name != "Turning the Undead" {
		t.Errorf("seventh trait = %q, want Turning the Undead", traits[6].Name)
	}
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
	}
}

func TestClassTraitsBard(t *testing.T) {
	traits := ClassTraits("Bard", 1)
	if len(traits) != 3 {
		t.Fatalf("expected 3 traits, got %d", len(traits))
	}
	if traits[0].Name != "Bard Skills" {
		t.Errorf("first trait = %q, want Bard Skills", traits[0].Name)
	}
	if traits[1].Name != "Counter Charm" {
		t.Errorf("second trait = %q, want Counter Charm", traits[1].Name)
	}
	if traits[2].Name != "Enchantment" {
		t.Errorf("third trait = %q, want Enchantment", traits[2].Name)
	}
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
	}
}

func TestClassTraitsEnchanter(t *testing.T) {
	traits := ClassTraits("Enchanter", 1)
	if len(traits) != 6 {
		t.Fatalf("expected 6 traits, got %d", len(traits))
	}
	if traits[0].Name != "Restrictions" {
		t.Errorf("first trait = %q, want Restrictions", traits[0].Name)
	}
	if traits[1].Name != "Enchanter Skills" {
		t.Errorf("second trait = %q, want Enchanter Skills", traits[1].Name)
	}
	if traits[5].Name != "Resistance to Divine Aid" {
		t.Errorf("sixth trait = %q, want Resistance to Divine Aid", traits[5].Name)
	}
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
	}
}

func TestClassTraitsFighter(t *testing.T) {
	traits := ClassTraits("Fighter", 1)
	if len(traits) != 1 {
		t.Fatalf("expected 1 trait, got %d", len(traits))
	}
	if traits[0].Name != "Combat Talents" {
		t.Errorf("first trait = %q, want Combat Talents", traits[0].Name)
	}
	if traits[0].Description == "" {
		t.Error("expected description for Combat Talents")
	}
}

func TestClassTraitsMagician(t *testing.T) {
	traits := ClassTraits("Magician", 1)
	if len(traits) != 4 {
		t.Fatalf("expected 4 traits, got %d", len(traits))
	}
	if traits[0].Name != "Arcane Magic" {
		t.Errorf("first trait = %q, want Arcane Magic", traits[0].Name)
	}
	if traits[1].Name != "Detect Magic" {
		t.Errorf("second trait = %q, want Detect Magic", traits[1].Name)
	}
	if traits[3].Name != "Spell Books" {
		t.Errorf("fourth trait = %q, want Spell Books", traits[3].Name)
	}
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
	}
}

func TestClassTraitsThief(t *testing.T) {
	traits := ClassTraits("Thief", 1)
	if len(traits) != 3 {
		t.Fatalf("expected 3 traits, got %d", len(traits))
	}
	if traits[0].Name != "Back-Stab" {
		t.Errorf("first trait = %q, want Back-Stab", traits[0].Name)
	}
	if traits[1].Name != "Thief Skills" {
		t.Errorf("second trait = %q, want Thief Skills", traits[1].Name)
	}
	if traits[2].Name != "Thieves' Cant" {
		t.Errorf("third trait = %q, want Thieves' Cant", traits[2].Name)
	}
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
	}
}

func TestClassTraitsHunter(t *testing.T) {
	traits := ClassTraits("Hunter", 1)
	if len(traits) != 5 {
		t.Fatalf("expected 5 traits, got %d", len(traits))
	}
	if traits[0].Name != "Animal Companion" {
		t.Errorf("first trait = %q, want Animal Companion", traits[0].Name)
	}
	if traits[1].Name != "Hunter Skills" {
		t.Errorf("second trait = %q, want Hunter Skills", traits[1].Name)
	}
	if traits[4].Name != "Wayfinding" {
		t.Errorf("fifth trait = %q, want Wayfinding", traits[4].Name)
	}
	for _, trait := range traits {
		if trait.Description == "" {
			t.Errorf("expected description for %s", trait.Name)
		}
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

