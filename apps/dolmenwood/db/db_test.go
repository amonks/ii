package db

import (
	"database/sql"
	"testing"
	"time"

	"monks.co/apps/dolmenwood/engine"
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
		Name:          "Sir Galahad",
		Class:         "Knight",
		Kindred:       "Human",
		Level:         1,
		STR:           16,
		DEX:           10,
		CON:           14,
		INT:           9,
		WIS:           12,
		CHA:           13,
		HPCurrent:     8,
		HPMax:         8,
		Alignment:     "Lawful",
		Background:    "Noble",
		Liege:         "Duke Maldric",
		BirthdayMonth: "Grimvold",
		BirthdayDay:   19,
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
	if got.BirthdayMonth != "Grimvold" {
		t.Errorf("BirthdayMonth = %q, want %q", got.BirthdayMonth, "Grimvold")
	}
	if got.BirthdayDay != 19 {
		t.Errorf("BirthdayDay = %d, want 19", got.BirthdayDay)
	}
}

func TestCharacterDefaults(t *testing.T) {
	db := newTestDB(t)

	type columnInfo struct {
		Name      string         `gorm:"column:name"`
		DfltValue sql.NullString `gorm:"column:dflt_value"`
	}

	var columns []columnInfo
	if err := db.Raw("PRAGMA table_info(characters)").Scan(&columns).Error; err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}

	defaults := map[string]sql.NullString{}
	for _, col := range columns {
		defaults[col.Name] = col.DfltValue
	}

	for _, name := range []string{"class", "kindred"} {
		if def, ok := defaults[name]; ok && def.Valid {
			t.Errorf("%s default = %q, want no default", name, def.String)
		}
	}
}

func TestListCharacters(t *testing.T) {
	db := newTestDB(t)

	if err := db.CreateCharacter(&Character{Name: "Alice", Class: "Knight", Kindred: "Human", Level: 1}); err != nil {
		t.Fatalf("CreateCharacter Alice: %v", err)
	}
	if err := db.CreateCharacter(&Character{Name: "Bob", Class: "Knight", Kindred: "Human", Level: 2}); err != nil {
		t.Fatalf("CreateCharacter Bob: %v", err)
	}

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
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

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
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

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

func TestCompanionDefaults(t *testing.T) {
	db := newTestDB(t)

	type columnInfo struct {
		Name      string         `gorm:"column:name"`
		DfltValue sql.NullString `gorm:"column:dflt_value"`
	}

	var columns []columnInfo
	if err := db.Raw("PRAGMA table_info(companions)").Scan(&columns).Error; err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}

	defaults := map[string]sql.NullString{}
	for _, col := range columns {
		defaults[col.Name] = col.DfltValue
	}

	loyalty, ok := defaults["loyalty"]
	if !ok {
		t.Fatal("expected loyalty column in companions")
	}
	if !loyalty.Valid || loyalty.String != "0" {
		t.Errorf("loyalty default = %q, want %q", loyalty.String, "0")
	}
}

func TestTransactions(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	tx := &Transaction{
		CharacterID:     ch.ID,
		Amount:          50,
		CoinType:        "gp",
		Description:     "dragon hoard",
		IsFoundTreasure: true,
		CreatedAt:       time.Now(),
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
		Name:    "Test",
		Class:   "Knight",
		Kindred: "Human",
		Level:   1,
		STR:     13, // prime ability
		FoundGP: 50,
		FoundSP: 100,
	}
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	// Return to Safety should:
	// 1. Move found treasure to purse
	// 2. Calculate XP from treasure (GP value * modifier)
	// 3. Create XP log entry
	// 4. Zero out found treasure
	// 5. Create audit log
	if err := db.ReturnToSafety(ch.ID, 15, 0); err != nil {
		t.Fatalf("ReturnToSafety: %v", err)
	}

	got, _ := db.GetCharacter(ch.ID)
	// Found treasure zeroed
	if got.FoundGP != 0 || got.FoundSP != 0 {
		t.Errorf("found treasure not zeroed: GP=%d SP=%d", got.FoundGP, got.FoundSP)
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
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

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
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

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
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

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
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

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
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

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
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	if err := db.AddAuditLog(ch.ID, "hp_change", "HP 8 -> 5", 0); err != nil {
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

func TestAuditLogGameDay(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	if err := db.AddAuditLog(ch.ID, "hp_change", "HP 8 -> 5", 3); err != nil {
		t.Fatalf("AddAuditLog: %v", err)
	}

	logs, _ := db.ListAuditLog(ch.ID)
	if len(logs) != 1 {
		t.Fatalf("len = %d, want 1", len(logs))
	}
	if logs[0].GameDay != 3 {
		t.Errorf("GameDay = %d, want 3", logs[0].GameDay)
	}
}

func TestCharacterCurrentDay(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1, CurrentDay: 1}
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	got, _ := db.GetCharacter(ch.ID)
	if got.CurrentDay != 1 {
		t.Errorf("CurrentDay = %d, want 1", got.CurrentDay)
	}

	got.CurrentDay = 5
	db.UpdateCharacter(got)

	got, _ = db.GetCharacter(ch.ID)
	if got.CurrentDay != 5 {
		t.Errorf("CurrentDay = %d, want 5", got.CurrentDay)
	}
}

func TestRetainerContracts(t *testing.T) {
	db := newTestDB(t)

	employer := &Character{Name: "Employer", Class: "Knight", Kindred: "Human", Level: 1}
	retainer := &Character{Name: "Retainer", Class: "Friar", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(employer); err != nil {
		t.Fatalf("CreateCharacter employer: %v", err)
	}
	if err := db.CreateCharacter(retainer); err != nil {
		t.Fatalf("CreateCharacter retainer: %v", err)
	}

	contract := &RetainerContract{
		EmployerID:   employer.ID,
		RetainerID:   retainer.ID,
		LootSharePct: 20,
		XPSharePct:   55,
		DailyWageCP:  12,
		HiredOnDay:   3,
		Active:       true,
	}
	if err := db.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}
	if contract.ID == 0 {
		t.Fatal("expected contract ID to be set")
	}

	got, err := db.GetRetainerContract(contract.ID)
	if err != nil {
		t.Fatalf("GetRetainerContract: %v", err)
	}
	if got.LootSharePct != 20 {
		t.Errorf("LootSharePct = %v, want 20", got.LootSharePct)
	}
	if got.XPSharePct != 55 {
		t.Errorf("XPSharePct = %v, want 55", got.XPSharePct)
	}
	if got.DailyWageCP != 12 {
		t.Errorf("DailyWageCP = %d, want 12", got.DailyWageCP)
	}
	if got.HiredOnDay != 3 {
		t.Errorf("HiredOnDay = %d, want 3", got.HiredOnDay)
	}
}

func TestListActiveRetainerContracts(t *testing.T) {
	db := newTestDB(t)

	employer := &Character{Name: "Employer", Class: "Knight", Kindred: "Human", Level: 1}
	retainer := &Character{Name: "Retainer", Class: "Friar", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(employer); err != nil {
		t.Fatalf("CreateCharacter employer: %v", err)
	}
	if err := db.CreateCharacter(retainer); err != nil {
		t.Fatalf("CreateCharacter retainer: %v", err)
	}

	contract := &RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := db.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	contracts, err := db.ListActiveRetainerContracts(employer.ID)
	if err != nil {
		t.Fatalf("ListActiveRetainerContracts: %v", err)
	}
	if len(contracts) != 1 {
		t.Fatalf("len = %d, want 1", len(contracts))
	}
	if contracts[0].ID != contract.ID {
		t.Errorf("ID = %d, want %d", contracts[0].ID, contract.ID)
	}
}

func TestDeactivateRetainerContract(t *testing.T) {
	db := newTestDB(t)

	employer := &Character{Name: "Employer", Class: "Knight", Kindred: "Human", Level: 1}
	retainer := &Character{Name: "Retainer", Class: "Friar", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(employer); err != nil {
		t.Fatalf("CreateCharacter employer: %v", err)
	}
	if err := db.CreateCharacter(retainer); err != nil {
		t.Fatalf("CreateCharacter retainer: %v", err)
	}

	contract := &RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := db.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	if err := db.DeactivateRetainerContract(contract.ID); err != nil {
		t.Fatalf("DeactivateRetainerContract: %v", err)
	}

	contracts, err := db.ListActiveRetainerContracts(employer.ID)
	if err != nil {
		t.Fatalf("ListActiveRetainerContracts: %v", err)
	}
	if len(contracts) != 0 {
		t.Fatalf("len = %d, want 0", len(contracts))
	}
}

func TestDeleteCharacterCleansRetainerContracts(t *testing.T) {
	db := newTestDB(t)

	employer := &Character{Name: "Employer", Class: "Knight", Kindred: "Human", Level: 1}
	retainer := &Character{Name: "Retainer", Class: "Friar", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(employer); err != nil {
		t.Fatalf("CreateCharacter employer: %v", err)
	}
	if err := db.CreateCharacter(retainer); err != nil {
		t.Fatalf("CreateCharacter retainer: %v", err)
	}

	contract := &RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := db.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	if err := db.DeleteCharacter(employer.ID); err != nil {
		t.Fatalf("DeleteCharacter employer: %v", err)
	}

	if _, err := db.GetCharacter(retainer.ID); err != nil {
		t.Fatalf("retainer should remain: %v", err)
	}

	contracts, err := db.ListActiveRetainerContracts(employer.ID)
	if err != nil {
		t.Fatalf("ListActiveRetainerContracts: %v", err)
	}
	if len(contracts) != 0 {
		t.Fatalf("len = %d, want 0", len(contracts))
	}
}

func TestDeleteRetainerCleansRetainerContracts(t *testing.T) {
	db := newTestDB(t)

	employer := &Character{Name: "Employer", Class: "Knight", Kindred: "Human", Level: 1}
	retainer := &Character{Name: "Retainer", Class: "Friar", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(employer); err != nil {
		t.Fatalf("CreateCharacter employer: %v", err)
	}
	if err := db.CreateCharacter(retainer); err != nil {
		t.Fatalf("CreateCharacter retainer: %v", err)
	}

	contract := &RetainerContract{EmployerID: employer.ID, RetainerID: retainer.ID, Active: true}
	if err := db.CreateRetainerContract(contract); err != nil {
		t.Fatalf("CreateRetainerContract: %v", err)
	}

	if err := db.DeleteCharacter(retainer.ID); err != nil {
		t.Fatalf("DeleteCharacter retainer: %v", err)
	}

	if _, err := db.GetCharacter(employer.ID); err != nil {
		t.Fatalf("employer should remain: %v", err)
	}

	contracts, err := db.ListActiveRetainerContracts(employer.ID)
	if err != nil {
		t.Fatalf("ListActiveRetainerContracts: %v", err)
	}
	if len(contracts) != 0 {
		t.Fatalf("len = %d, want 0", len(contracts))
	}
}

func TestBankDeposits(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Knight", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	dep := &BankDeposit{
		CharacterID: ch.ID,
		CoinNotes:   "50gp",
		CPValue:     5000,
		DepositDay:  1,
	}
	if err := db.CreateBankDeposit(dep); err != nil {
		t.Fatalf("CreateBankDeposit: %v", err)
	}
	if dep.ID == 0 {
		t.Fatal("expected ID to be set")
	}

	deps, err := db.ListBankDeposits(ch.ID)
	if err != nil {
		t.Fatalf("ListBankDeposits: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("len = %d, want 1", len(deps))
	}
	if deps[0].CPValue != 5000 {
		t.Errorf("CPValue = %d, want 5000", deps[0].CPValue)
	}
	if deps[0].CoinNotes != "50gp" {
		t.Errorf("CoinNotes = %q, want %q", deps[0].CoinNotes, "50gp")
	}

	// Update
	deps[0].CPValue = 4000
	if err := db.UpdateBankDeposit(&deps[0]); err != nil {
		t.Fatalf("UpdateBankDeposit: %v", err)
	}
	deps, _ = db.ListBankDeposits(ch.ID)
	if deps[0].CPValue != 4000 {
		t.Errorf("after update CPValue = %d, want 4000", deps[0].CPValue)
	}

	// Delete
	if err := db.DeleteBankDeposit(dep.ID); err != nil {
		t.Fatalf("DeleteBankDeposit: %v", err)
	}
	deps, _ = db.ListBankDeposits(ch.ID)
	if len(deps) != 0 {
		t.Errorf("after delete len = %d, want 0", len(deps))
	}
}

func TestPreparedSpellsCRUD(t *testing.T) {
	db := newTestDB(t)

	ch := &Character{Name: "Test", Class: "Magician", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(ch); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	spell := &PreparedSpell{
		CharacterID: ch.ID,
		Name:        "Fairy Servant",
		SpellLevel:  1,
		Used:        false,
	}
	if err := db.CreatePreparedSpell(spell); err != nil {
		t.Fatalf("CreatePreparedSpell: %v", err)
	}
	if spell.ID == 0 {
		t.Fatal("expected spell ID to be set")
	}

	spells, err := db.ListPreparedSpells(ch.ID)
	if err != nil {
		t.Fatalf("ListPreparedSpells: %v", err)
	}
	if len(spells) != 1 {
		t.Fatalf("len = %d, want 1", len(spells))
	}
	if spells[0].Name != spell.Name || spells[0].SpellLevel != 1 || spells[0].Used {
		t.Fatalf("unexpected spell data: %+v", spells[0])
	}

	if err := db.MarkSpellUsed(spell.ID); err != nil {
		t.Fatalf("MarkSpellUsed: %v", err)
	}
	spells, _ = db.ListPreparedSpells(ch.ID)
	if !spells[0].Used {
		t.Fatalf("expected spell to be marked used")
	}

	if err := db.ResetSpells(ch.ID); err != nil {
		t.Fatalf("ResetSpells: %v", err)
	}
	spells, _ = db.ListPreparedSpells(ch.ID)
	if spells[0].Used {
		t.Fatalf("expected spell to be reset")
	}

	if err := db.DeletePreparedSpell(spell.ID); err != nil {
		t.Fatalf("DeletePreparedSpell: %v", err)
	}
	spells, _ = db.ListPreparedSpells(ch.ID)
	if len(spells) != 0 {
		t.Fatalf("len = %d, want 0", len(spells))
	}
}

func TestTransferItemFull(t *testing.T) {
	db := newTestDB(t)

	source := &Character{Name: "Source", Class: "Knight", Kindred: "Human", Level: 1}
	target := &Character{Name: "Target", Class: "Knight", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(source); err != nil {
		t.Fatalf("CreateCharacter source: %v", err)
	}
	if err := db.CreateCharacter(target); err != nil {
		t.Fatalf("CreateCharacter target: %v", err)
	}

	container := &Item{CharacterID: source.ID, Name: "Backpack", Quantity: 1}
	if err := db.CreateItem(container); err != nil {
		t.Fatalf("CreateItem container: %v", err)
	}
	companion := &Companion{CharacterID: source.ID, Name: "Bessie", Breed: "Mule"}
	if err := db.CreateCompanion(companion); err != nil {
		t.Fatalf("CreateCompanion: %v", err)
	}

	item := &Item{
		CharacterID: source.ID,
		Name:        engine.CoinItemNameStr,
		Quantity:    5,
		Notes:       "5gp",
		ContainerID: &container.ID,
		CompanionID: &companion.ID,
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	source.CoinContainerID = &container.ID
	source.CoinCompanionID = &companion.ID
	if err := db.UpdateCharacter(source); err != nil {
		t.Fatalf("UpdateCharacter: %v", err)
	}

	if err := db.TransferItem(item.ID, target.ID, 0); err != nil {
		t.Fatalf("TransferItem: %v", err)
	}

	moved, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if moved.CharacterID != target.ID {
		t.Errorf("CharacterID = %d, want %d", moved.CharacterID, target.ID)
	}
	if moved.ContainerID != nil || moved.CompanionID != nil {
		t.Errorf("expected container/companion to be cleared, got %v/%v", moved.ContainerID, moved.CompanionID)
	}

	reloaded, _ := db.GetCharacter(source.ID)
	if reloaded.CoinContainerID != nil || reloaded.CoinCompanionID != nil {
		t.Errorf("expected coin references to clear, got %v/%v", reloaded.CoinContainerID, reloaded.CoinCompanionID)
	}
}

func TestTransferItemPartial(t *testing.T) {
	db := newTestDB(t)

	source := &Character{Name: "Source", Class: "Knight", Kindred: "Human", Level: 1}
	target := &Character{Name: "Target", Class: "Knight", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(source); err != nil {
		t.Fatalf("CreateCharacter source: %v", err)
	}
	if err := db.CreateCharacter(target); err != nil {
		t.Fatalf("CreateCharacter target: %v", err)
	}

	weight := 250
	item := &Item{
		CharacterID:    source.ID,
		Name:           "Torches",
		Notes:          "oil-soaked",
		Quantity:       5,
		IsTiny:         true,
		WeightOverride: &weight,
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	if err := db.TransferItem(item.ID, target.ID, 2); err != nil {
		t.Fatalf("TransferItem: %v", err)
	}

	updated, _ := db.GetItem(item.ID)
	if updated.Quantity != 3 {
		t.Errorf("source quantity = %d, want 3", updated.Quantity)
	}

	targetItems, _ := db.ListItems(target.ID)
	if len(targetItems) != 1 {
		t.Fatalf("target items = %d, want 1", len(targetItems))
	}
	moved := targetItems[0]
	if moved.Quantity != 2 {
		t.Errorf("moved quantity = %d, want 2", moved.Quantity)
	}
	if moved.ContainerID != nil || moved.CompanionID != nil {
		t.Errorf("expected moved container/companion nil, got %v/%v", moved.ContainerID, moved.CompanionID)
	}
	if moved.Name != item.Name || moved.Notes != item.Notes || !moved.IsTiny {
		t.Errorf("moved fields not copied")
	}
	if moved.WeightOverride == nil || *moved.WeightOverride != weight {
		t.Errorf("moved weight override = %v, want %d", moved.WeightOverride, weight)
	}
}

func TestTransferItemContainerMovesChildren(t *testing.T) {
	db := newTestDB(t)

	source := &Character{Name: "Source", Class: "Knight", Kindred: "Human", Level: 1}
	target := &Character{Name: "Target", Class: "Knight", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(source); err != nil {
		t.Fatalf("CreateCharacter source: %v", err)
	}
	if err := db.CreateCharacter(target); err != nil {
		t.Fatalf("CreateCharacter target: %v", err)
	}

	container := &Item{CharacterID: source.ID, Name: "Backpack", Quantity: 1}
	if err := db.CreateItem(container); err != nil {
		t.Fatalf("CreateItem container: %v", err)
	}
	child := &Item{CharacterID: source.ID, Name: "Rope", Quantity: 1, ContainerID: &container.ID}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("CreateItem child: %v", err)
	}

	if err := db.TransferItem(container.ID, target.ID, 0); err != nil {
		t.Fatalf("TransferItem: %v", err)
	}

	movedChild, _ := db.GetItem(child.ID)
	if movedChild.CharacterID != target.ID {
		t.Errorf("child CharacterID = %d, want %d", movedChild.CharacterID, target.ID)
	}
	if movedChild.ContainerID == nil || *movedChild.ContainerID != container.ID {
		t.Errorf("child container id = %v, want %d", movedChild.ContainerID, container.ID)
	}
}

func TestTransferItemContainerClearsCoinLocation(t *testing.T) {
	db := newTestDB(t)

	source := &Character{Name: "Source", Class: "Knight", Kindred: "Human", Level: 1}
	target := &Character{Name: "Target", Class: "Knight", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(source); err != nil {
		t.Fatalf("CreateCharacter source: %v", err)
	}
	if err := db.CreateCharacter(target); err != nil {
		t.Fatalf("CreateCharacter target: %v", err)
	}

	container := &Item{CharacterID: source.ID, Name: "Backpack", Quantity: 1}
	if err := db.CreateItem(container); err != nil {
		t.Fatalf("CreateItem container: %v", err)
	}

	coins := &Item{
		CharacterID: source.ID,
		Name:        engine.CoinItemNameStr,
		Quantity:    5,
		Notes:       "5gp",
		ContainerID: &container.ID,
	}
	if err := db.CreateItem(coins); err != nil {
		t.Fatalf("CreateItem coins: %v", err)
	}

	source.CoinContainerID = &container.ID
	if err := db.UpdateCharacter(source); err != nil {
		t.Fatalf("UpdateCharacter: %v", err)
	}

	if err := db.TransferItem(container.ID, target.ID, 0); err != nil {
		t.Fatalf("TransferItem: %v", err)
	}

	reloaded, _ := db.GetCharacter(source.ID)
	if reloaded.CoinContainerID != nil || reloaded.CoinCompanionID != nil {
		t.Errorf("expected coin references to clear, got %v/%v", reloaded.CoinContainerID, reloaded.CoinCompanionID)
	}

	movedCoins, _ := db.GetItem(coins.ID)
	if movedCoins.CharacterID != target.ID {
		t.Errorf("coin CharacterID = %d, want %d", movedCoins.CharacterID, target.ID)
	}
	if movedCoins.ContainerID == nil || *movedCoins.ContainerID != container.ID {
		t.Errorf("coin container id = %v, want %d", movedCoins.ContainerID, container.ID)
	}
}

func TestTransferItemCoinsPartial(t *testing.T) {
	db := newTestDB(t)

	source := &Character{Name: "Source", Class: "Knight", Kindred: "Human", Level: 1}
	target := &Character{Name: "Target", Class: "Knight", Kindred: "Human", Level: 1}
	if err := db.CreateCharacter(source); err != nil {
		t.Fatalf("CreateCharacter source: %v", err)
	}
	if err := db.CreateCharacter(target); err != nil {
		t.Fatalf("CreateCharacter target: %v", err)
	}

	item := &Item{CharacterID: source.ID, Name: engine.CoinItemNameStr, Quantity: 5, Notes: "3gp 2sp"}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	if err := db.TransferItem(item.ID, target.ID, 2); err != nil {
		t.Fatalf("TransferItem: %v", err)
	}

	updated, _ := db.GetItem(item.ID)
	if updated.Notes != "1gp 2sp" || updated.Quantity != 3 {
		t.Errorf("remaining notes/qty = %q/%d, want %q/3", updated.Notes, updated.Quantity, "1gp 2sp")
	}

	targetItems, _ := db.ListItems(target.ID)
	if len(targetItems) != 1 {
		t.Fatalf("target items = %d, want 1", len(targetItems))
	}
	if targetItems[0].Notes != "2gp" || targetItems[0].Quantity != 2 {
		t.Errorf("target notes/qty = %q/%d, want %q/2", targetItems[0].Notes, targetItems[0].Quantity, "2gp")
	}
}
