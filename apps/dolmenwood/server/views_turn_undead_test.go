package server

import (
	"testing"

	"monks.co/apps/dolmenwood/db"
)

func TestCharacterViewTurnUndeadTable(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{Name: "Clara", Class: "Cleric", Kindred: "Human", Level: 9, HPCurrent: 6, HPMax: 6}
	if err := d.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	if len(view.TurnUndeadTable) == 0 {
		t.Fatalf("expected turn undead table")
	}
	if view.TurnUndeadTable[0].Target != "T" {
		t.Errorf("turn undead target = %q, want T", view.TurnUndeadTable[0].Target)
	}
}

func TestCharacterViewNoTurnUndeadTable(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{Name: "Nope", Class: "Bard", Kindred: "Human", Level: 3, HPCurrent: 6, HPMax: 6}
	if err := d.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	if view.TurnUndeadTable != nil {
		t.Errorf("expected nil turn undead table, got %v", view.TurnUndeadTable)
	}
}
