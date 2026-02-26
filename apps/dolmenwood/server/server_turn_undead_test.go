package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"monks.co/apps/dolmenwood/db"
)

func TestSheetShowsTurnUndeadTable(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{Name: "Cleric", Class: "Cleric", Kindred: "Human", Level: 9, HPCurrent: 6, HPMax: 6}
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
	if !strings.Contains(body, "Turn Undead") {
		t.Errorf("response should include Turn Undead section")
	}
	if !strings.Contains(body, "HD 1") {
		t.Errorf("response should include turn undead table headers")
	}
}
