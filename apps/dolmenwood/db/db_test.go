package db

import (
	"testing"
	"time"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	return db
}

func TestCharacterRoundTrip(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{
		Name:       "Sir Galahad",
		Class:      "Knight",
		Kindred:    "Human",
		Level:      1,
		STR:        16,
		DEX:        10,
		CON:        14,
		INT:        9,
		WIS:        12,
		CHA:        13,
		HPCurrent:  8,
		HPMax:      8,
		Alignment:  "Lawful",
		Background: "Noble",
		Liege:      "Duke Maldric",
	}
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}
	if ch.ID == 0 {
		t.Fatal("expected ID to be set after create")
	}

	got, err := db.GetCharacter(ch.ID)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	if got.Name != "Sir Galahad" {
		t.Errorf("Name = %q, want %q", got.Name, "Sir Galahad")
	}
	if got.STR != 16 {
		t.Errorf("STR = %d, want 16", got.STR)
	}
}

func TestListCharacters(t *testing.T) {
	db := newTestDB(t)

	db.CreateCharacter(&Character{Name: "Alice", Class: "Knight", Kindred: "Human", Level: 1})
	db.CreateCharacter(&Character{Name: "Bob", Class: "Knight", Kindred: "Human", Level: 2})

	chars, err := db.ListCharacters()
	if err != nil {
		t.Fatalf("ListCharacters: %v", err)
	}
	if len(chars) != 2 {
		t.Errorf("len = %d, want 2", len(chars))
	}
}

func TestItems(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	db.CreateCharacter(ch)

	item := &Item{
		CharacterID: ch.ID,
		Name:        "Longsword",
		Quantity:    1,
		Location:    "equipped",
		SortOrder:   0,
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	items, err := db.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len = %d, want 1", len(items))
	}
	if items[0].Name != "Longsword" {
		t.Errorf("Name = %q, want %q", items[0].Name, "Longsword")
	}
}

func TestCompanion(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	db.CreateCharacter(ch)

	comp := &Companion{
		CharacterID: ch.ID,
		Name:        "Thunder",
		Breed:       "Charger",
		HPCurrent:   22,
		HPMax:       22,
	}
	if err := db.CreateCompanion(comp); err != nil {
		t.Fatalf("CreateCompanion: %v", err)
	}

	// Update HP
	comp.HPCurrent = 18
	if err := db.UpdateCompanion(comp); err != nil {
		t.Fatalf("UpdateCompanion: %v", err)
	}

	comps, err := db.ListCompanions(ch.ID)
	if err != nil {
		t.Fatalf("ListCompanions: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("len = %d, want 1", len(comps))
	}
	if comps[0].HPCurrent != 18 {
		t.Errorf("HPCurrent = %d, want 18", comps[0].HPCurrent)
	}
}

func TestTransactions(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	db.CreateCharacter(ch)

	tx := &Transaction{
		CharacterID:    ch.ID,
		Amount:         50,
		CoinType:       "gp",
		Description:    "dragon hoard",
		IsFoundTreasure: true,
		CreatedAt:      time.Now(),
	}
	if err := db.CreateTransaction(tx); err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}

	txs, err := db.ListTransactions(ch.ID)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("len = %d, want 1", len(txs))
	}
	if !txs[0].IsFoundTreasure {
		t.Error("IsFoundTreasure = false, want true")
	}
}

func TestReturnToSafety(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{
		Name:     "Test",
		Class:    "Knight",
		Kindred:  "Human",
		Level:    1,
		STR:      13, // prime ability
		FoundGP:  50,
		FoundSP:  100,
		PurseGP:  10,
	}
	db.CreateCharacter(ch)

	// Return to Safety should:
	// 1. Move found treasure to purse
	// 2. Calculate XP from treasure (GP value * modifier)
	// 3. Create XP log entry
	// 4. Zero out found treasure
	// 5. Create audit log
	if err := db.ReturnToSafety(ch.ID, 15); err != nil {
		t.Fatalf("ReturnToSafety: %v", err)
	}

	got, _ := db.GetCharacter(ch.ID)
	// Found treasure zeroed
	if got.FoundGP != 0 || got.FoundSP != 0 {
		t.Errorf("found treasure not zeroed: GP=%d SP=%d", got.FoundGP, got.FoundSP)
	}
	// Purse updated
	if got.PurseGP != 60 {
		t.Errorf("PurseGP = %d, want 60 (10 + 50)", got.PurseGP)
	}
	if got.PurseSP != 100 {
		t.Errorf("PurseSP = %d, want 100", got.PurseSP)
	}

	// XP log entry created
	xpLogs, _ := db.ListXPLog(ch.ID)
	if len(xpLogs) != 1 {
		t.Fatalf("XP log entries = %d, want 1", len(xpLogs))
	}
	// 50 GP + 100 SP (=10 GP) = 60 GP value base XP
	// 60 * 1.15 = 69 XP (with +15% modifier)
	if xpLogs[0].Amount != 69 {
		t.Errorf("XP amount = %d, want 69", xpLogs[0].Amount)
	}

	// Audit log entry
	audits, _ := db.ListAuditLog(ch.ID)
	if len(audits) == 0 {
		t.Error("expected audit log entries")
	}
}

func TestNotes(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	db.CreateCharacter(ch)

	note := &Note{
		CharacterID: ch.ID,
		Content:     "Remember to visit the blacksmith",
		CreatedAt:   time.Now(),
	}
	if err := db.CreateNote(note); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	notes, err := db.ListNotes(ch.ID)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("len = %d, want 1", len(notes))
	}
	if notes[0].Content != "Remember to visit the blacksmith" {
		t.Errorf("Content = %q", notes[0].Content)
	}
}

func TestItemContainerHierarchy(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	db.CreateCharacter(ch)

	// Create a backpack
	backpack := &Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1, Location: "equipped"}
	db.CreateItem(backpack)

	// Create an item inside the backpack
	rope := &Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1, ContainerID: &backpack.ID}
	db.CreateItem(rope)

	items, err := db.ListItems(ch.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	// Find the rope and check its container_id
	for _, it := range items {
		if it.Name == "Rope" {
			if it.ContainerID == nil {
				t.Error("Rope ContainerID should not be nil")
			} else if *it.ContainerID != backpack.ID {
				t.Errorf("Rope ContainerID = %d, want %d", *it.ContainerID, backpack.ID)
			}
		}
	}
}

func TestItemOnCompanion(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	db.CreateCharacter(ch)

	comp := &Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	db.CreateCompanion(comp)

	bedroll := &Item{CharacterID: ch.ID, Name: "Bedroll", Quantity: 1, CompanionID: &comp.ID}
	db.CreateItem(bedroll)

	items, _ := db.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].CompanionID == nil || *items[0].CompanionID != comp.ID {
		t.Errorf("Bedroll CompanionID = %v, want %d", items[0].CompanionID, comp.ID)
	}
}

func TestDeleteContainerCascade(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	db.CreateCharacter(ch)

	backpack := &Item{CharacterID: ch.ID, Name: "Backpack", Quantity: 1}
	db.CreateItem(backpack)

	rope := &Item{CharacterID: ch.ID, Name: "Rope", Quantity: 1, ContainerID: &backpack.ID}
	db.CreateItem(rope)

	// Delete the backpack - children should be reparented to nil (equipped on character)
	if err := db.DeleteItem(backpack.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}

	items, _ := db.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (rope)", len(items))
	}
	if items[0].Name != "Rope" {
		t.Errorf("remaining item = %q, want Rope", items[0].Name)
	}
	if items[0].ContainerID != nil {
		t.Errorf("Rope ContainerID = %v, want nil (reparented)", items[0].ContainerID)
	}
}

func TestDeleteCompanionMovesItems(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	db.CreateCharacter(ch)

	comp := &Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	db.CreateCompanion(comp)

	bedroll := &Item{CharacterID: ch.ID, Name: "Bedroll", Quantity: 1, CompanionID: &comp.ID}
	db.CreateItem(bedroll)

	if err := db.DeleteCompanion(comp.ID); err != nil {
		t.Fatalf("DeleteCompanion: %v", err)
	}

	items, _ := db.ListItems(ch.ID)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].CompanionID != nil {
		t.Errorf("Bedroll CompanionID = %v, want nil", items[0].CompanionID)
	}
}

func TestAuditLog(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	db.CreateCharacter(ch)

	if err := db.AddAuditLog(ch.ID, "hp_change", "HP 8 -> 5"); err != nil {
		t.Fatalf("AddAuditLog: %v", err)
	}

	logs, err := db.ListAuditLog(ch.ID)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("len = %d, want 1", len(logs))
	}
	if logs[0].Action != "hp_change" {
		t.Errorf("Action = %q, want %q", logs[0].Action, "hp_change")
	}
}

