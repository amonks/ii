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
