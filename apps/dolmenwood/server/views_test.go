package server

import (
	"testing"

	"monks.co/apps/dolmenwood/db"
)

func TestWealthViewAggregatesCoinItems(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
		FoundGP: 50,
	}
	d.CreateCharacter(ch)

	// Create consolidated coin items in various locations
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 280, Notes: "80gp 200sp"})

	comp := &db.Companion{CharacterID: ch.ID, Name: "Bessie", Breed: "Mule", HPCurrent: 9, HPMax: 9}
	d.CreateCompanion(comp)
	d.CreateItem(&db.Item{CharacterID: ch.ID, Name: "Coins", Quantity: 70, Notes: "70gp", CompanionID: &comp.ID})

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	// Should aggregate coins from all locations by parsing notes
	if view.InventoryCoins["gp"] != 150 {
		t.Errorf("InventoryCoins[gp] = %d, want 150", view.InventoryCoins["gp"])
	}
	if view.InventoryCoins["sp"] != 200 {
		t.Errorf("InventoryCoins[sp] = %d, want 200", view.InventoryCoins["sp"])
	}

	// GP value: 150 GP + 200/10 SP = 170 GP
	if view.InventoryGPValue != 170 {
		t.Errorf("InventoryGPValue = %d, want 170", view.InventoryGPValue)
	}

	// Purse = inventory - found: 170 - 50 = 120 GP
	if view.PurseGPValue != 120 {
		t.Errorf("PurseGPValue = %d, want 120 (inventory 170 - found 50)", view.PurseGPValue)
	}
	if view.FoundGPValue != 50 {
		t.Errorf("FoundGPValue = %d, want 50", view.FoundGPValue)
	}

	// Per-denomination purse should be inventory minus found
	if view.PurseCoins["gp"] != 100 {
		t.Errorf("PurseCoins[gp] = %d, want 100 (150 - 50 found)", view.PurseCoins["gp"])
	}
	if view.PurseCoins["sp"] != 200 {
		t.Errorf("PurseCoins[sp] = %d, want 200 (200 - 0 found)", view.PurseCoins["sp"])
	}
}

func TestSaveBonusesInView(t *testing.T) {
	_, d := setupTest(t)

	// Mossling Knight born under Maiden's moon Full should have save bonuses from
	// kindred (Resilience), class (Strength of Will), and moon sign.
	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Mossling",
		Level: 1, HPCurrent: 8, HPMax: 8,
		BirthdayMonth: "Chysting", BirthdayDay: 18, // Maiden's moon Full
	}
	d.CreateCharacter(ch)

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	sources := map[string]bool{}
	for _, b := range view.SaveBonuses {
		sources[b.Source] = true
	}
	if !sources["Resilience"] {
		t.Error("expected Resilience save bonus for Mossling")
	}
	if !sources["Strength of Will"] {
		t.Error("expected Strength of Will save bonus for Knight")
	}
	if !sources["Moon Sign"] {
		t.Error("expected Moon Sign save bonus for Maiden's moon Full")
	}
}

func TestSaveBonusesInViewNoBirthday(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{
		Name: "Test", Class: "Fighter", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	if len(view.SaveBonuses) != 0 {
		t.Errorf("expected no save bonuses for Human Fighter with no birthday, got %d", len(view.SaveBonuses))
	}
}

func TestCompanionStatsFromItems(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{
		CharacterID: ch.ID, Name: "Bessie", Breed: "Mule",
		HPCurrent: 9, HPMax: 9,
	}
	d.CreateCompanion(comp)

	// Add pack saddle and barding as items on companion
	d.CreateItem(&db.Item{
		CharacterID: ch.ID, Name: "Pack saddle and bridle",
		Quantity: 1, CompanionID: &comp.ID,
	})
	d.CreateItem(&db.Item{
		CharacterID: ch.ID, Name: "Horse barding",
		Quantity: 1, CompanionID: &comp.ID,
	})

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	if len(view.Companions) != 1 {
		t.Fatalf("expected 1 companion, got %d", len(view.Companions))
	}
	cv := view.Companions[0]

	// Pack saddle should give full breed capacity (25 for Mule)
	if cv.LoadCapacity != 25 {
		t.Errorf("LoadCapacity = %d, want 25 (pack saddle on Mule)", cv.LoadCapacity)
	}

	// Barding should give +2 AC (12 + 2 = 14)
	if cv.AC != 14 {
		t.Errorf("AC = %d, want 14 (base 12 + barding 2)", cv.AC)
	}
}

func TestCompanionStatsNoGear(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{
		CharacterID: ch.ID, Name: "Bessie", Breed: "Mule",
		HPCurrent: 9, HPMax: 9,
	}
	d.CreateCompanion(comp)

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	cv := view.Companions[0]

	// No saddle = 0 capacity
	if cv.LoadCapacity != 0 {
		t.Errorf("LoadCapacity = %d, want 0 (no saddle)", cv.LoadCapacity)
	}

	// No barding = base AC
	if cv.AC != 12 {
		t.Errorf("AC = %d, want 12 (no barding)", cv.AC)
	}
}

func TestItemIsTiny(t *testing.T) {
	tests := []struct {
		name string
		item InventoryItem
		want bool
	}{
		{
			name: "explicit tiny flag",
			item: InventoryItem{Item: db.Item{Name: "Clothes", IsTiny: true}},
			want: true,
		},
		{
			name: "built-in tiny item",
			item: InventoryItem{Item: db.Item{Name: "Bell"}},
			want: true,
		},
		{
			name: "clothing is not tiny",
			item: InventoryItem{Item: db.Item{Name: "Clothes"}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := itemIsTiny(tt.item); got != tt.want {
				t.Errorf("itemIsTiny(%q) = %t, want %t", tt.item.Name, got, tt.want)
			}
		})
	}
}

func TestCompanionGearShowsZeroSlotsInView(t *testing.T) {
	_, d := setupTest(t)

	ch := &db.Character{
		Name: "Test", Class: "Knight", Kindred: "Human",
		Level: 1, HPCurrent: 8, HPMax: 8,
	}
	d.CreateCharacter(ch)

	comp := &db.Companion{
		CharacterID: ch.ID, Name: "Bessie", Breed: "Mule",
		HPCurrent: 9, HPMax: 9,
	}
	d.CreateCompanion(comp)

	// Add a Pack Saddle and Bridle (companion gear) to the companion
	d.CreateItem(&db.Item{
		CharacterID: ch.ID, Name: "Pack Saddle and Bridle", Quantity: 1,
		CompanionID: &comp.ID,
	})

	view, err := buildCharacterView(d, ch)
	if err != nil {
		t.Fatalf("buildCharacterView: %v", err)
	}

	// Find the saddle in companion groups
	for _, cg := range view.CompanionGroups {
		for _, item := range cg.Items {
			if item.Name == "Pack Saddle and Bridle" {
				if item.Slots != 0 {
					t.Errorf("companion gear Pack Saddle and Bridle should have Slots=0, got %d", item.Slots)
				}
				return
			}
		}
	}
	t.Fatal("Pack Saddle and Bridle not found in companion inventory")
}
