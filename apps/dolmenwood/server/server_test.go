package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"monks.co/apps/dolmenwood/db"
)

func setupTest(t *testing.T) (*Server, *db.DB) {
	t.Helper()
	d, err := db.NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	srv := New(d)
	return srv, d
}

func TestGetIndex(t *testing.T) {
	srv, _ := setupTest(t)
	mux := srv.Mux()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Dolmenwood") {
		t.Error("response should contain 'Dolmenwood'")
	}
}

func TestCreateCharacter(t *testing.T) {
	srv, _ := setupTest(t)
	mux := srv.Mux()

	form := url.Values{}
	form.Set("name", "Sir Galahad")
	form.Set("str", "16")
	form.Set("dex", "10")
	form.Set("con", "14")
	form.Set("int", "9")
	form.Set("wis", "12")
	form.Set("cha", "13")
	form.Set("hp_max", "8")
	form.Set("armor_name", "Chain Mail")
	form.Set("armor_ac", "14")
	form.Set("alignment", "Lawful")
	form.Set("background", "Noble")
	form.Set("liege", "Duke Maldric")

	req := httptest.NewRequest("POST", "/characters/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	loc := w.Header().Get("Location")
	if loc != "1/" {
		t.Errorf("Location = %q, want %q", loc, "1/")
	}
}

func TestGetCharacterSheet(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Sir Galahad", Class: "Knight", Kindred: "Human",
		Level: 1, STR: 16, DEX: 10, CON: 14, INT: 9, WIS: 12, CHA: 13,
		HPCurrent: 8, HPMax: 8, ArmorName: "Chain Mail", ArmorAC: 14,
	}
	d.CreateCharacter(ch)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Sir Galahad") {
		t.Error("response should contain character name")
	}
}

func TestUpdateHP(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("hp_current", "5")
	req := httptest.NewRequest("POST", "/characters/1/hp/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify HP was updated in DB
	got, _ := d.GetCharacter(ch.ID)
	if got.HPCurrent != 5 {
		t.Errorf("HPCurrent = %d, want 5", got.HPCurrent)
	}
}
