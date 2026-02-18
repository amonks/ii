package engine

import "testing"

func TestSpeedFromSlots(t *testing.T) {
	cases := []struct {
		name          string
		equippedSlots int
		stowedSlots   int
		want          int
	}{
		{"empty", 0, 0, 40},
		{"light load", 3, 10, 40},
		{"equipped threshold", 4, 10, 30},
		{"stowed threshold", 3, 11, 30},
		{"medium load", 6, 13, 20},
		{"heavy load", 8, 15, 10},
		{"mixed — takes slower", 4, 14, 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SpeedFromSlots(tc.equippedSlots, tc.stowedSlots)
			if got != tc.want {
				t.Errorf("SpeedFromSlots(%d, %d) = %d, want %d", tc.equippedSlots, tc.stowedSlots, got, tc.want)
			}
		})
	}
}

func TestCoinSlots(t *testing.T) {
	cases := []struct {
		coins int
		want  int
	}{
		{0, 0},
		{1, 1},
		{100, 1},
		{101, 2},
		{200, 2},
		{201, 3},
	}
	for _, tc := range cases {
		got := CoinSlots(tc.coins)
		if got != tc.want {
			t.Errorf("CoinSlots(%d) = %d, want %d", tc.coins, got, tc.want)
		}
	}
}

func TestTotalSlotsWithCatalog(t *testing.T) {
	items := []Item{
		// Equipped: longsword (30cn → 1), shield (100cn → 1), dagger (10cn → 1) = 3 slots
		{Name: "Longsword", Quantity: 1, Location: "equipped"},
		{Name: "Shield", Quantity: 1, Location: "equipped"},
		{Name: "Dagger", Quantity: 1, Location: "equipped"},
		// Stowed: 5 preserved rations (100cn → 1), rope (100cn → 1) = 2 slots
		{Name: "Preserved Rations", Quantity: 5, Location: "stowed"},
		{Name: "Rope", Quantity: 1, Location: "stowed"},
		// Horse: ignored by equipped/stowed
		{Name: "Bedroll", Quantity: 1, Location: "horse"},
	}

	equipped := TotalEquippedSlots(items)
	if equipped != 3 {
		t.Errorf("TotalEquippedSlots = %d, want 3", equipped)
	}

	stowed := TotalStowedSlots(items)
	if stowed != 2 {
		t.Errorf("TotalStowedSlots = %d, want 2", stowed)
	}
}

func TestRationsEncumbrance(t *testing.T) {
	// 10 preserved rations at 20cn each = 200cn = 2 slots
	items := []Item{
		{Name: "Preserved Rations", Quantity: 10, Location: "stowed"},
	}
	got := TotalStowedSlots(items)
	if got != 2 {
		t.Errorf("10 rations = %d slots, want 2", got)
	}
}

func TestWeightOverride(t *testing.T) {
	w := 150
	items := []Item{
		{Name: "Custom Thing", Quantity: 1, Location: "stowed", WeightOverride: &w},
	}
	got := TotalStowedSlots(items)
	if got != 2 {
		t.Errorf("150cn custom item = %d slots, want 2", got)
	}
}

func TestUnknownItemWeightless(t *testing.T) {
	items := []Item{
		{Name: "Magic Thingamajig", Quantity: 1, Location: "stowed"},
	}
	got := TotalStowedSlots(items)
	if got != 0 {
		t.Errorf("unknown item = %d slots, want 0", got)
	}
}

func TestItemSlots(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want int
	}{
		{"single ration", Item{Name: "Preserved Rations", Quantity: 1, Location: "stowed"}, 1},
		{"5 rations", Item{Name: "Preserved Rations", Quantity: 5, Location: "stowed"}, 1},
		{"10 rations", Item{Name: "Preserved Rations", Quantity: 10, Location: "stowed"}, 2},
		{"longsword", Item{Name: "Longsword", Quantity: 1, Location: "equipped"}, 1},
		{"unknown", Item{Name: "Magic Orb", Quantity: 1, Location: "equipped"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ItemSlots(tc.item)
			if got != tc.want {
				t.Errorf("ItemSlots = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEquippedContainerFreeSlots(t *testing.T) {
	items := []Item{
		{Name: "Backpack", Quantity: 1, Location: "equipped"},
		{Name: "Longsword", Quantity: 1, Location: "equipped"},
	}
	// Backpack equipped = 0 slots, longsword = 1 slot
	got := TotalEquippedSlots(items)
	if got != 1 {
		t.Errorf("TotalEquippedSlots = %d, want 1 (backpack should be free)", got)
	}
}

func TestStowedCapacity(t *testing.T) {
	items := []Item{
		{Name: "Backpack", Quantity: 1, Location: "equipped"},
	}
	cap, containers := StowedCapacity(items)
	if cap != 10 {
		t.Errorf("StowedCapacity = %d, want 10", cap)
	}
	if len(containers) != 1 {
		t.Fatalf("got %d containers, want 1", len(containers))
	}
	if containers[0].Name != "Backpack" {
		t.Errorf("container name = %q, want %q", containers[0].Name, "Backpack")
	}
	if containers[0].Slots != 10 {
		t.Errorf("container slots = %d, want 10", containers[0].Slots)
	}
}

func TestStowedCapacityMultiple(t *testing.T) {
	items := []Item{
		{Name: "Backpack", Quantity: 1, Location: "equipped"},
		{Name: "Sack", Quantity: 1, Location: "equipped"},
	}
	cap, _ := StowedCapacity(items)
	// Backpack 10 + Sack 10 = 20, capped at 16
	if cap != 16 {
		t.Errorf("StowedCapacity = %d, want 16 (capped)", cap)
	}
}

func TestStowedContainerNotEquipped(t *testing.T) {
	// A backpack that's stowed (not in use) provides no capacity
	items := []Item{
		{Name: "Backpack", Quantity: 1, Location: "stowed"},
	}
	cap, containers := StowedCapacity(items)
	if cap != 0 {
		t.Errorf("StowedCapacity = %d, want 0 (backpack not equipped)", cap)
	}
	if len(containers) != 0 {
		t.Errorf("got %d containers, want 0", len(containers))
	}
}

func TestTotalHorseSlots(t *testing.T) {
	items := []Item{
		{Name: "Bedroll", Quantity: 1, Location: "horse"},
		{Name: "Preserved Rations", Quantity: 10, Location: "horse"},
		{Name: "Longsword", Quantity: 1, Location: "stowed"}, // not on horse
	}
	got := TotalHorseSlots(items)
	// Bedroll 70cn → 1 slot, 10 rations 200cn → 2 slots = 3
	if got != 3 {
		t.Errorf("TotalHorseSlots = %d, want 3", got)
	}
}
