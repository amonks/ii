package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"monks.co/apps/dolmenwood/db"
)

func TestCharacterViewSpellSlots(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{Name: "Tessa", Class: "Cleric", Kindred: "Human", Level: 4, HPCurrent: 5, HPMax: 5}
	if err := d.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}
	spell := &db.PreparedSpell{CharacterID: ch.ID, Name: "Bless", SpellLevel: 1}
	if err := d.CreatePreparedSpell(spell); err != nil {
		t.Fatalf("CreatePreparedSpell: %v", err)
	}
	usedSpell := &db.PreparedSpell{CharacterID: ch.ID, Name: "Light", SpellLevel: 2, Used: true}
	if err := d.CreatePreparedSpell(usedSpell); err != nil {
		t.Fatalf("CreatePreparedSpell: %v", err)
	}

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}
	if view.SpellSlots == nil {
		t.Fatal("expected spell slots")
	}
	if view.SpellSlots.Level1 != 2 || view.SpellSlots.Level2 != 1 {
		t.Fatalf("SpellSlots = %+v, want L1=2 L2=1", view.SpellSlots)
	}
	if view.AvailableSpellSlots == nil {
		t.Fatal("expected available spell slots")
	}
	if view.AvailableSpellSlots.Level1 != 1 || view.AvailableSpellSlots.Level2 != 0 {
		t.Fatalf("AvailableSpellSlots = %+v, want L1=1 L2=0", view.AvailableSpellSlots)
	}
	if len(view.PreparedSpells) != 2 {
		t.Fatalf("PreparedSpells = %d, want 2", len(view.PreparedSpells))
	}
}

func TestSpellHandlersLifecycle(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{Name: "Merla", Class: "Magician", Kindred: "Human", Level: 3, HPCurrent: 4, HPMax: 4}
	if err := d.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	form := url.Values{"name": {"Sleep"}, "spell_level": {"1"}}
	req := httptest.NewRequest("POST", "/characters/1/spells/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "Sleep") {
		t.Errorf("response should include prepared spell name")
	}

	spells, err := d.ListPreparedSpells(ch.ID)
	if err != nil {
		t.Fatalf("ListPreparedSpells: %v", err)
	}
	if len(spells) != 1 {
		t.Fatalf("prepared spell count = %d, want 1", len(spells))
	}
	spellID := spells[0].ID

	req = httptest.NewRequest("POST", fmt.Sprintf("/characters/1/spells/%d/cast/", spellID), nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cast status = %d, want %d", w.Code, http.StatusOK)
	}
	spells, _ = d.ListPreparedSpells(ch.ID)
	if !spells[0].Used {
		t.Fatalf("expected spell to be marked used")
	}

	req = httptest.NewRequest("POST", "/characters/1/spells/rest/", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("rest status = %d, want %d", w.Code, http.StatusOK)
	}
	spells, _ = d.ListPreparedSpells(ch.ID)
	if spells[0].Used {
		t.Fatalf("expected spell to be reset to unused")
	}

	req = httptest.NewRequest("POST", fmt.Sprintf("/characters/1/spells/%d/forget/", spellID), nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("forget status = %d, want %d", w.Code, http.StatusOK)
	}
	spells, _ = d.ListPreparedSpells(ch.ID)
	if len(spells) != 0 {
		t.Fatalf("expected prepared spell to be deleted")
	}
}

func TestPrepareSpellValidatesSlots(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{Name: "Etta", Class: "Magician", Kindred: "Human", Level: 2, HPCurrent: 4, HPMax: 4}
	if err := d.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	form := url.Values{"name": {"Charm"}, "spell_level": {"2"}}
	req := httptest.NewRequest("POST", "/characters/1/spells/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("level 2 status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	spells, err := d.ListPreparedSpells(ch.ID)
	if err != nil {
		t.Fatalf("ListPreparedSpells: %v", err)
	}
	if len(spells) != 0 {
		t.Fatalf("prepared spell count = %d, want 0", len(spells))
	}

	for i := range 2 {
		form = url.Values{"name": {fmt.Sprintf("Spell %d", i+1)}, "spell_level": {"1"}}
		req = httptest.NewRequest("POST", "/characters/1/spells/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("prepare %d status = %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	form = url.Values{"name": {"Overflow"}, "spell_level": {"1"}}
	req = httptest.NewRequest("POST", "/characters/1/spells/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("overflow status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	spells, err = d.ListPreparedSpells(ch.ID)
	if err != nil {
		t.Fatalf("ListPreparedSpells: %v", err)
	}
	if len(spells) != 2 {
		t.Fatalf("prepared spell count = %d, want 2", len(spells))
	}
}
