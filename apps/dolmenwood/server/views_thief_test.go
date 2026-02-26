package server

import (
	"testing"

	"monks.co/apps/dolmenwood/db"
)

func TestCharacterViewThiefBackstabAndSkills(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{Name: "Sneak", Class: "Thief", Kindred: "Human", Level: 3, HPCurrent: 4, HPMax: 4}
	if err := d.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	if view.ThiefBackstabBonus != 4 {
		t.Errorf("ThiefBackstabBonus = %d, want 4", view.ThiefBackstabBonus)
	}
	if view.ThiefBackstabDamage != "3d4" {
		t.Errorf("ThiefBackstabDamage = %q, want 3d4", view.ThiefBackstabDamage)
	}
	if len(view.ThiefSkillTargets) == 0 {
		t.Fatal("expected thief skill targets")
	}
	if view.ThiefSkillTargets["Climb Wall"] != 4 {
		t.Errorf("Climb Wall target = %d, want 4", view.ThiefSkillTargets["Climb Wall"])
	}
	if len(view.ThiefSkillNames) == 0 {
		t.Errorf("expected thief skill names")
	}
}

func TestCharacterViewNonThiefHasNoSkills(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{Name: "Fighter", Class: "Fighter", Kindred: "Human", Level: 1, HPCurrent: 8, HPMax: 8}
	if err := d.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	if len(view.ThiefSkillTargets) != 0 {
		t.Errorf("expected no thief skill targets, got %d", len(view.ThiefSkillTargets))
	}
	if view.ThiefBackstabBonus != 0 || view.ThiefBackstabDamage != "" {
		t.Errorf("expected empty backstab stats, got bonus=%d damage=%q", view.ThiefBackstabBonus, view.ThiefBackstabDamage)
	}
}
