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
		HPCurrent: 8, HPMax: 8,
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

func TestACDerivedFromEquippedItems(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, DEX: 10, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Add chainmail as equipped item
	d.CreateItem(&db.Item{
		CharacterID: ch.ID,
		Name:        "Chainmail",
		Quantity:    1,
		Location:    "equipped",
	})

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	// AC 14 = chainmail base AC 14 + DEX 10 modifier 0
	if !strings.Contains(body, ">14<") {
		t.Error("response should show AC 14 for equipped chainmail")
	}
	if !strings.Contains(body, "Chainmail") {
		t.Error("response should show armor name 'Chainmail'")
	}
}

func TestWeaponDamageInStatBlock(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, DEX: 10, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Add longsword as equipped weapon
	d.CreateItem(&db.Item{
		CharacterID: ch.ID,
		Name:        "Longsword",
		Quantity:    1,
		Location:    "equipped",
	})

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Longsword") {
		t.Error("response should show equipped weapon name")
	}
	if !strings.Contains(body, "1d8") {
		t.Error("response should show weapon damage 1d8")
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
	form.Set("name", "tiny lock of hair")
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
	if items[0].Name != "lock of hair" {
		t.Errorf("Name = %q, want %q", items[0].Name, "lock of hair")
	}
	if !items[0].IsTiny {
		t.Error("expected item to be marked tiny")
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
	// Mule: HP 9 (stored), AC/Speed/Load derived from breed
	if got.HPMax != 9 {
		t.Errorf("HPMax = %d, want 9", got.HPMax)
	}
	if got.HPCurrent != 9 {
		t.Errorf("HPCurrent = %d, want 9", got.HPCurrent)
	}
}

func TestCompanionStatsDerivedFromBreed(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Add a Charger companion
	form := url.Values{}
	form.Set("name", "Warhorse")
	form.Set("breed", "Charger")
	req := httptest.NewRequest("POST", "/characters/1/companions/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	// Should show breed-derived stats: AC 12, Speed 40, Load 40
	if !strings.Contains(body, "Charger") {
		t.Error("response should contain breed name")
	}
	// Should show saves
	if !strings.Contains(body, "Death") {
		t.Error("response should show save labels")
	}
	// Charger attack: "2 hooves (+2, 1d6)"
	if !strings.Contains(body, "hooves") {
		t.Error("response should show attack info")
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
		CharacterID: ch.ID,
		Name:        "Old Horse",
		Breed:       "Mule",
		HPCurrent:   9,
		HPMax:       9,
	}
	d.CreateCompanion(comp)

	form := url.Values{}
	form.Set("name", "Shadowfax")
	form.Set("hp_current", "7")
	form.Set("hp_max", "12")
	form.Set("saddle_type", "pack")
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
	if got.HPCurrent != 7 {
		t.Errorf("HPCurrent = %d, want 7", got.HPCurrent)
	}
	if got.HPMax != 12 {
		t.Errorf("HPMax = %d, want 12", got.HPMax)
	}
	if got.SaddleType != "pack" {
		t.Errorf("SaddleType = %q, want %q", got.SaddleType, "pack")
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
		CharacterID: ch.ID,
		Name:        "Old Nag",
		Breed:       "Mule",
		HPCurrent:   9,
		HPMax:       9,
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

func TestMoveItemToContainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	// Create a backpack (equipped on character)
	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	// Create a rope (equipped on character)
	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1}
	d.CreateItem(rope)

	// Move rope into backpack via move_to
	form := url.Values{}
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", rope.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	for _, it := range items {
		if it.Name == "Rope" {
			if it.ContainerID == nil || *it.ContainerID != backpack.ID {
				t.Errorf("Rope ContainerID = %v, want %d", it.ContainerID, backpack.ID)
			}
		}
	}
}

func TestMoveBundledItemAutoCombines(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	existing := &db.Item{CharacterID: ch.ID, Name: "Torch", Quantity: 2, ContainerID: &backpack.ID}
	d.CreateItem(existing)

	loose := &db.Item{CharacterID: ch.ID, Name: "Torch", Quantity: 2}
	d.CreateItem(loose)

	form := url.Values{}
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", loose.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	var torchCount int
	for _, it := range items {
		if it.Name == "Torch" {
			torchCount++
			if it.Quantity != 4 {
				t.Errorf("Torch Quantity = %d, want 4", it.Quantity)
			}
			if it.ContainerID == nil || *it.ContainerID != backpack.ID {
				t.Errorf("Torch ContainerID = %v, want %d", it.ContainerID, backpack.ID)
			}
		}
	}
	if torchCount != 1 {
		t.Fatalf("got %d torch items, want 1", torchCount)
	}
}

func TestMoveItemToCompanion(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1}
	d.CreateItem(rope)

	form := url.Values{}
	form.Set("move_to", fmt.Sprintf("companion:%d", comp.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", rope.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if items[0].CompanionID == nil || *items[0].CompanionID != comp.ID {
		t.Errorf("Rope CompanionID = %v, want %d", items[0].CompanionID, comp.ID)
	}
}

func TestMoveItemToEquipped(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1, ContainerID: &backpack.ID}
	d.CreateItem(rope)

	// Move rope back to equipped
	form := url.Values{}
	form.Set("move_to", "equipped")
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", rope.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	for _, it := range items {
		if it.Name == "Rope" {
			if it.ContainerID != nil {
				t.Errorf("Rope ContainerID = %v, want nil", it.ContainerID)
			}
			if it.CompanionID != nil {
				t.Errorf("Rope CompanionID = %v, want nil", it.CompanionID)
			}
		}
	}
}

func TestMoveItemToCompanionContainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	// Create a chest on the companion
	chest := &db.Item{CharacterID: ch.ID, Name: "Chest (wooden, large)", Quantity: 1, CompanionID: &comp.ID}
	d.CreateItem(chest)

	// Create rope equipped on character
	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1}
	d.CreateItem(rope)

	// Move rope into the chest (which is on the companion)
	form := url.Values{}
	form.Set("move_to", fmt.Sprintf("container:%d", chest.ID))
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/update/", rope.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	for _, it := range items {
		if it.Name == "Rope" {
			if it.ContainerID == nil || *it.ContainerID != chest.ID {
				t.Errorf("Rope ContainerID = %v, want %d", it.ContainerID, chest.ID)
			}
			if it.CompanionID != nil {
				t.Errorf("Rope CompanionID = %v, want nil (it's in a container, not directly on companion)", it.CompanionID)
			}
		}
	}
}

func TestAddItemToContainer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	form := url.Values{}
	form.Set("name", "Rope")
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	for _, it := range items {
		if it.Name == "Rope" {
			if it.ContainerID == nil || *it.ContainerID != backpack.ID {
				t.Errorf("Rope ContainerID = %v, want %d", it.ContainerID, backpack.ID)
			}
		}
	}
}

func TestAddBundledItemAutoCombines(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	existing := &db.Item{CharacterID: ch.ID, Name: "Torch", Quantity: 2, ContainerID: &backpack.ID}
	d.CreateItem(existing)

	form := url.Values{}
	form.Set("name", "Torch")
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	var found bool
	for _, it := range items {
		if it.Name == "Torch" {
			found = true
			if it.Quantity != 3 {
				t.Errorf("Torch Quantity = %d, want 3", it.Quantity)
			}
			if it.ContainerID == nil || *it.ContainerID != backpack.ID {
				t.Errorf("Torch ContainerID = %v, want %d", it.ContainerID, backpack.ID)
			}
		}
	}
	if !found {
		t.Fatal("expected torch item to remain")
	}
}

func TestAddBundledItemInLocationAutoCombines(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	existing := &db.Item{CharacterID: ch.ID, Name: "Torch", Quantity: 2, Location: "stowed"}
	d.CreateItem(existing)

	form := url.Values{}
	form.Set("name", "Torch")
	form.Set("location", "stowed")
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
	if items[0].Quantity != 3 {
		t.Errorf("Torch Quantity = %d, want 3", items[0].Quantity)
	}
	if items[0].Location != "stowed" {
		t.Errorf("Torch Location = %q, want stowed", items[0].Location)
	}
}

func TestAddUnbundledItemDoesNotCombine(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	existing := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1, ContainerID: &backpack.ID}
	d.CreateItem(existing)

	form := url.Values{}
	form.Set("name", "Rope")
	form.Set("move_to", fmt.Sprintf("container:%d", backpack.ID))
	req := httptest.NewRequest("POST", "/characters/1/items/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	var ropes int
	for _, it := range items {
		if it.Name == "Rope" {
			ropes++
		}
	}
	if ropes != 2 {
		t.Fatalf("got %d ropes, want 2", ropes)
	}
}

func TestDecrementBundledItem(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	torches := &db.Item{CharacterID: ch.ID, Name: "Torches", Quantity: 6}
	d.CreateItem(torches)

	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/decrement/", torches.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Quantity != 3 {
		t.Errorf("Torches Quantity = %d, want 3", items[0].Quantity)
	}
}

func TestBundledItemUsesDecrementButton(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	torches := &db.Item{CharacterID: ch.ID, Name: "Torches", Quantity: 3}
	d.CreateItem(torches)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, fmt.Sprintf("items/%d/decrement/", torches.ID)) {
		t.Fatalf("expected decrement button for bundled item")
	}
	if strings.Contains(body, fmt.Sprintf("items/%d/delete/", torches.ID)) {
		t.Fatalf("expected bundled item to avoid delete button")
	}
}

func TestUnbundledItemUsesDeleteButton(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1}
	d.CreateItem(rope)

	req := httptest.NewRequest("GET", "/characters/1/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, fmt.Sprintf("items/%d/delete/", rope.ID)) {
		t.Fatalf("expected delete button for unbundled item")
	}
	if strings.Contains(body, fmt.Sprintf("items/%d/decrement/", rope.ID)) {
		t.Fatalf("expected unbundled item to avoid decrement button")
	}
}

func TestDeleteContainerCascadeServer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	rope := &db.Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1, ContainerID: &backpack.ID}
	d.CreateItem(rope)

	// Delete backpack
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/items/%d/delete/", backpack.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (rope remains)", len(items))
	}
	if items[0].Name != "Rope" {
		t.Errorf("remaining item = %q, want Rope", items[0].Name)
	}
	if items[0].ContainerID != nil {
		t.Errorf("Rope ContainerID = %v, want nil", items[0].ContainerID)
	}
}

func TestDeleteCompanionMovesItemsServer(t *testing.T) {
	srv, d := setupTest(t)
	mux := srv.Mux()

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	bedroll := &db.Item{CharacterID: ch.ID, Name: "Bedroll", Quantity: 1, CompanionID: &comp.ID}
	d.CreateItem(bedroll)

	// Delete companion
	req := httptest.NewRequest("POST", fmt.Sprintf("/characters/1/companions/%d/delete/", comp.ID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Companion should be gone
	comps, _ := d.ListCompanions(ch.ID)
	if len(comps) != 0 {
		t.Errorf("got %d companions, want 0", len(comps))
	}

	// Item should remain, moved to equipped on character
	items, _ := d.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].CompanionID != nil {
		t.Errorf("Bedroll CompanionID = %v, want nil", items[0].CompanionID)
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

func TestBuildMoveTargetsIncludesCompanionContainers(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)

	// Backpack equipped on character
	backpack := &db.Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	d.CreateItem(backpack)

	// Chest on the companion
	chest := &db.Item{CharacterID: ch.ID, Name: "Chest (wooden, large)", Quantity: 1, CompanionID: &comp.ID}
	d.CreateItem(chest)

	items, _ := d.ListItems(ch.ID)
	compViews := []CompanionView{{Companion: *comp, LoadCapacity: 25}}
	targets := buildMoveTargets(items, compViews)

	// Should have: Equipped, Backpack, Chest (Bessie), Bessie (Mule)
	found := map[string]bool{}
	for _, t := range targets {
		found[t.Label] = true
	}

	if !found["Equipped"] {
		t.Error("missing Equipped target")
	}
	if !found["Backpack"] {
		t.Error("missing Backpack target")
	}
	if !found["Chest (wooden, large) (Bessie)"] {
		t.Errorf("missing companion container target, got targets: %v", targets)
	}
	if !found["Bessie (Mule)"] {
		t.Errorf("missing companion target, got targets: %v", targets)
	}
}

func TestEquippedSpeedChart(t *testing.T) {
	cells := EquippedSpeedChart(5)
	if len(cells) != 10 {
		t.Fatalf("got %d cells, want 10", len(cells))
	}
	// First 5 should be filled, rest empty
	for i, c := range cells {
		if i < 5 && !c.Filled {
			t.Errorf("cell %d should be filled", i)
		}
		if i >= 5 && c.Filled {
			t.Errorf("cell %d should be empty", i)
		}
	}
	// Speed brackets: 0-2=40, 3-4=30, 5-6=20, 7-9=10
	if cells[0].Speed != 40 {
		t.Errorf("cell 0 speed = %d, want 40", cells[0].Speed)
	}
	if cells[3].Speed != 30 {
		t.Errorf("cell 3 speed = %d, want 30", cells[3].Speed)
	}
	if cells[5].Speed != 20 {
		t.Errorf("cell 5 speed = %d, want 20", cells[5].Speed)
	}
	if cells[7].Speed != 10 {
		t.Errorf("cell 7 speed = %d, want 10", cells[7].Speed)
	}
}

func TestStowedSpeedChart(t *testing.T) {
	cells := StowedSpeedChart(12)
	if len(cells) != 16 {
		t.Fatalf("got %d cells, want 16", len(cells))
	}
	// First 12 filled, rest empty
	for i, c := range cells {
		if i < 12 && !c.Filled {
			t.Errorf("cell %d should be filled", i)
		}
		if i >= 12 && c.Filled {
			t.Errorf("cell %d should be empty", i)
		}
	}
	// Speed brackets: 0-9=40, 10-11=30, 12-13=20, 14-15=10
	if cells[0].Speed != 40 {
		t.Errorf("cell 0 speed = %d, want 40", cells[0].Speed)
	}
	if cells[10].Speed != 30 {
		t.Errorf("cell 10 speed = %d, want 30", cells[10].Speed)
	}
	if cells[12].Speed != 20 {
		t.Errorf("cell 12 speed = %d, want 20", cells[12].Speed)
	}
	if cells[14].Speed != 10 {
		t.Errorf("cell 14 speed = %d, want 10", cells[14].Speed)
	}
}
