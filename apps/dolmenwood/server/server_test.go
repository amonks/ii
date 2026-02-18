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

func TestAddTinyItem(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("name", "Brass Key")
	form.Set("location", "stowed")
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Name != "Brass Key" {
		t.Errorf("Name = %q, want %q", items[0].Name, "Brass Key")
	}
}

func TestMoveItemToHorse(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	item := &db.Item{
		CharacterID: ch.ID,
		Name:        "Rope",
		Quantity:    1,
		Location:    "stowed",
	}
	d.CreateItem(item)

	// Move to horse
	form := url.Values{}
	form.Set("location", "horse")
	req := httptest.NewRequest("POST", "/characters/1/items/1/update/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if items[0].Location != "horse" {
		t.Errorf("Location = %q, want %q", items[0].Location, "horse")
	}
}

func TestAddItemToHorse(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("name", "Tent")
	form.Set("location", "horse")
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Location != "horse" {
		t.Errorf("Location = %q, want %q", items[0].Location, "horse")
	}
}

func TestAddCompanionByBreed(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("name", "Bessie")
	form.Set("breed", "Mule")
	req := httptest.NewRequest("POST", "/characters/1/companions/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	comps, _ := d.ListCompanions(ch.ID)
	if len(comps) != 1 {
		t.Fatalf("got %d companions, want 1", len(comps))
	}
	got := comps[0]
	if got.Name != "Bessie" {
		t.Errorf("Name = %q, want %q", got.Name, "Bessie")
	}
	if got.Breed != "Mule" {
		t.Errorf("Breed = %q, want %q", got.Breed, "Mule")
	}
	// Mule stats: AC 12, HP 9, Speed 40, Load 25
	if got.AC != 12 {
		t.Errorf("AC = %d, want 12", got.AC)
	}
	if got.HPMax != 9 {
		t.Errorf("HPMax = %d, want 9", got.HPMax)
	}
	if got.HPCurrent != 9 {
		t.Errorf("HPCurrent = %d, want 9", got.HPCurrent)
	}
	if got.Speed != 40 {
		t.Errorf("Speed = %d, want 40", got.Speed)
	}
	if got.LoadCapacity != 25 {
		t.Errorf("LoadCapacity = %d, want 25", got.LoadCapacity)
	}
}

func TestAddXP(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("xp_amount", "100")
	form.Set("description", "Quest reward")
	req := httptest.NewRequest("POST", "/characters/1/xp/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify XP was added to character
	got, _ := d.GetCharacter(ch.ID)
	if got.TotalXP != 100 {
		t.Errorf("TotalXP = %d, want 100", got.TotalXP)
	}

	// Verify XP log entry
	xpLog, err := d.ListXPLog(ch.ID)
	if err != nil {
		t.Fatalf("ListXPLog: %v", err)
	}
	if len(xpLog) != 1 {
		t.Fatalf("got %d XP log entries, want 1", len(xpLog))
	}
	if xpLog[0].Amount != 100 {
		t.Errorf("XPLogEntry.Amount = %d, want 100", xpLog[0].Amount)
	}
	if xpLog[0].Description != "Quest reward" {
		t.Errorf("XPLogEntry.Description = %q, want %q", xpLog[0].Description, "Quest reward")
	}

	// Verify audit log
	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(auditLog) != 1 {
		t.Fatalf("got %d audit log entries, want 1", len(auditLog))
	}
	if auditLog[0].Action != "xp_add" {
		t.Errorf("AuditLog.Action = %q, want %q", auditLog[0].Action, "xp_add")
	}
}

func TestUpdateCompanion(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{
		CharacterID:  ch.ID,
		Name:         "Old Horse",
		Breed:        "Pony",
		HPCurrent:    10,
		HPMax:        10,
		AC:           13,
		Speed:        60,
		LoadCapacity: 40,
	}
	d.CreateCompanion(comp)

	form := url.Values{}
	form.Set("name", "Shadowfax")
	form.Set("breed", "Warhorse")
	form.Set("hp_current", "7")
	form.Set("hp_max", "12")
	form.Set("ac", "15")
	form.Set("speed", "90")
	form.Set("load_capacity", "50")
	form.Set("has_saddlebags", "on")
	form.Set("has_barding", "on")
	req := httptest.NewRequest("POST", "/characters/1/companions/1/update/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	comps, err := d.ListCompanions(ch.ID)
	if err != nil {
		t.Fatalf("ListCompanions: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("got %d companions, want 1", len(comps))
	}
	got := comps[0]
	if got.Name != "Shadowfax" {
		t.Errorf("Name = %q, want %q", got.Name, "Shadowfax")
	}
	if got.Breed != "Warhorse" {
		t.Errorf("Breed = %q, want %q", got.Breed, "Warhorse")
	}
	if got.HPCurrent != 7 {
		t.Errorf("HPCurrent = %d, want 7", got.HPCurrent)
	}
	if got.HPMax != 12 {
		t.Errorf("HPMax = %d, want 12", got.HPMax)
	}
	if got.AC != 15 {
		t.Errorf("AC = %d, want 15", got.AC)
	}
	if got.Speed != 90 {
		t.Errorf("Speed = %d, want 90", got.Speed)
	}
	if got.LoadCapacity != 50 {
		t.Errorf("LoadCapacity = %d, want 50", got.LoadCapacity)
	}
	if !got.HasSaddlebags {
		t.Error("HasSaddlebags should be true")
	}
	if !got.HasBarding {
		t.Error("HasBarding should be true")
	}
}

func TestUndoTransaction(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
		PurseGP: 50,
	}
	d.CreateCharacter(ch)

	// Create a transaction to undo
	tx := &db.Transaction{
		CharacterID:     ch.ID,
		Amount:          50,
		CoinType:        "gp",
		Description:     "dragon hoard",
		IsFoundTreasure: false,
	}
	d.CreateTransaction(tx)

	// Undo it
	req := httptest.NewRequest("POST", "/characters/1/treasure/1/undo/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify inverse transaction was created
	txs, err := d.ListTransactions(ch.ID)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("got %d transactions, want 2", len(txs))
	}
	// Most recent first
	undo := txs[0]
	if undo.Amount != -50 {
		t.Errorf("undo Amount = %d, want -50", undo.Amount)
	}
	if undo.CoinType != "gp" {
		t.Errorf("undo CoinType = %q, want %q", undo.CoinType, "gp")
	}
	if undo.Description != "undo dragon hoard" {
		t.Errorf("undo Description = %q, want %q", undo.Description, "undo dragon hoard")
	}

	// Verify coins were reversed
	got, _ := d.GetCharacter(ch.ID)
	if got.PurseGP != 0 {
		t.Errorf("PurseGP = %d, want 0", got.PurseGP)
	}
}

func TestUndoFoundTransaction(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
		FoundSP: 100,
	}
	d.CreateCharacter(ch)

	tx := &db.Transaction{
		CharacterID:     ch.ID,
		Amount:          100,
		CoinType:        "sp",
		Description:     "loot",
		IsFoundTreasure: true,
	}
	d.CreateTransaction(tx)

	req := httptest.NewRequest("POST", "/characters/1/treasure/1/undo/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	got, _ := d.GetCharacter(ch.ID)
	if got.FoundSP != 0 {
		t.Errorf("FoundSP = %d, want 0", got.FoundSP)
	}
}

func TestDeleteCompanion(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{
		CharacterID:  ch.ID,
		Name:         "Old Nag",
		Breed:        "Pony",
		HPCurrent:    10,
		HPMax:        10,
		AC:           13,
		Speed:        60,
		LoadCapacity: 40,
	}
	d.CreateCompanion(comp)

	req := httptest.NewRequest("POST", "/characters/1/companions/1/delete/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	comps, _ := d.ListCompanions(ch.ID)
	if len(comps) != 0 {
		t.Errorf("got %d companions, want 0", len(comps))
	}
}

func TestAddItemWithQuantityPrefix(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	form := url.Values{}
	form.Set("name", "5x Preserved Rations")
	form.Set("location", "stowed")
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, err := d.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Name != "Preserved Rations" {
		t.Errorf("Name = %q, want %q", items[0].Name, "Preserved Rations")
	}
	if items[0].Quantity != 5 {
		t.Errorf("Quantity = %d, want 5", items[0].Quantity)
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
