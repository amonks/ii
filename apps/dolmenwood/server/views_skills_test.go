package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"monks.co/apps/dolmenwood/db"
)

func TestCharacterViewBardSkillTargets(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{Name: "Lyric", Class: "Bard", Kindred: "Human", Level: 8, HPCurrent: 6, HPMax: 6}
	if err := d.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	if view.BardSkillTargets["Monster Lore"] != 3 {
		t.Errorf("Monster Lore target = %d, want 3", view.BardSkillTargets["Monster Lore"])
	}
	if len(view.BardSkillNames) == 0 {
		t.Errorf("expected bard skill names")
	}
	if view.EnchantmentUsesTotal != 8 {
		t.Errorf("EnchantmentUsesTotal = %d, want 8", view.EnchantmentUsesTotal)
	}
	if view.EnchantmentUsesUsed != 0 {
		t.Errorf("EnchantmentUsesUsed = %d, want 0", view.EnchantmentUsesUsed)
	}
	if view.EnchantmentUsesRemaining != 8 {
		t.Errorf("EnchantmentUsesRemaining = %d, want 8", view.EnchantmentUsesRemaining)
	}
	if view.GlamoursKnown != 0 {
		t.Errorf("GlamoursKnown = %d, want 0", view.GlamoursKnown)
	}
}

func TestCharacterViewHunterSkillTargets(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{Name: "Scout", Class: "Hunter", Kindred: "Human", Level: 8, HPCurrent: 8, HPMax: 8}
	if err := d.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	if view.HunterSkillTargets["Tracking"] != 3 {
		t.Errorf("Tracking target = %d, want 3", view.HunterSkillTargets["Tracking"])
	}
	if len(view.HunterSkillNames) == 0 {
		t.Errorf("expected hunter skill names")
	}
}

func TestSheetShowsBardAndHunterSkills(t *testing.T) {
	cases := []struct {
		name      string
		class     string
		expected  string
		skillName string
	}{
		{name: "bard", class: "Bard", expected: "Bard Skills", skillName: "Monster Lore"},
		{name: "hunter", class: "Hunter", expected: "Hunter Skills", skillName: "Tracking"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, d := setupTest(t)
			mux := srv.Mux()

			ch := &db.Character{Name: "Test", Class: tc.class, Kindred: "Human", Level: 8, HPCurrent: 8, HPMax: 8}
			if err := d.CreateCharacter(ch); err != nil {
				t.Fatalf("CreateCharacter: %v", err)
			}

			req := httptest.NewRequest("GET", "/characters/1/", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}
			body := w.Body.String()
			if !strings.Contains(body, tc.expected) {
				t.Errorf("response should include %q", tc.expected)
			}
			if !strings.Contains(body, tc.skillName) {
				t.Errorf("response should include %q", tc.skillName)
			}
		})
	}
}
