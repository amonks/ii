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
		// Equipped: longsword (1 slot), shield (1 slot), dagger (1 slot) = 3 slots
		{Name: "Longsword", Quantity: 1, Location: "equipped"},
		{Name: "Shield", Quantity: 1, Location: "equipped"},
		{Name: "Dagger", Quantity: 1, Location: "equipped"},
		// Stowed: 5 rations (5*20cn=100cn=1 slot), rope (100cn=1 slot) = 2 slots
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
	// 10 preserved rations: 10 * 20cn = 200cn = 2 slots (weight-based)
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

func TestUnknownItemDefaultsToOneSlot(t *testing.T) {
	items := []Item{
		{Name: "Magic Thingamajig", Quantity: 1, Location: "stowed"},
	}
	got := TotalStowedSlots(items)
	if got != 1 {
		t.Errorf("unknown item = %d slots, want 1", got)
	}
}

func TestItemSlots(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want int
	}{
		// Rations use weight-based: 20cn each
		{"single ration", Item{Name: "Preserved Rations", Quantity: 1, Location: "stowed"}, 1},
		{"5 rations = 1 slot", Item{Name: "Preserved Rations", Quantity: 5, Location: "stowed"}, 1},
		{"10 rations = 2 slots", Item{Name: "Preserved Rations", Quantity: 10, Location: "stowed"}, 2},
		// Armor/weapon/clothing use slot-based
		{"longsword", Item{Name: "Longsword", Quantity: 1, Location: "equipped"}, 1},
		{"unknown defaults to 1", Item{Name: "Magic Orb", Quantity: 1, Location: "equipped"}, 1},
		{"tiny unknown custom", Item{Name: "Lock of Hair", Quantity: 1, Location: "equipped", IsTiny: true}, 0},
		{"plate mail = 3 slots", Item{Name: "Plate mail", Quantity: 1, Location: "equipped"}, 3},
		{"chainmail = 2 slots", Item{Name: "Chainmail", Quantity: 1, Location: "equipped"}, 2},
		{"leather = 1 slot", Item{Name: "Leather", Quantity: 1, Location: "equipped"}, 1},
		{"shield = 1 slot", Item{Name: "Shield", Quantity: 1, Location: "equipped"}, 1},
		{"polearm = 2 slots", Item{Name: "Polearm", Quantity: 1, Location: "equipped"}, 2},
		{"two-handed sword = 2 slots", Item{Name: "Two-Handed Sword", Quantity: 1, Location: "equipped"}, 2},
		{"clothes = 0 slots", Item{Name: "Clothes", Quantity: 1, Location: "equipped"}, 0},
		// General items with known weight use weight-based
		{"rope = 1 slot", Item{Name: "Rope", Quantity: 1, Location: "stowed"}, 1},
		{"3 oil flasks = 1 slot", Item{Name: "Oil Flask", Quantity: 3, Location: "stowed"}, 1},
		{"bedroll = 1 slot", Item{Name: "Bedroll", Quantity: 1, Location: "stowed"}, 1},
		// Heavy containers: bulky = 2 slots each
		{"chest (wooden, large) = 2 slots", Item{Name: "Chest (wooden, large)", Quantity: 1, Location: "equipped"}, 2},
		{"casket (iron, large) = 2 slots", Item{Name: "Casket (iron, large)", Quantity: 1, Location: "equipped"}, 2},
		// Personal containers: 0 when equipped
		{"backpack equipped = 0 slots", Item{Name: "Backpack", Quantity: 1, Location: "equipped"}, 0},
		{"sack equipped = 0 slots", Item{Name: "Sack", Quantity: 1, Location: "equipped"}, 0},
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

func TestBundledItemSlots(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want int
	}{
		{"3 torches = 1 slot", Item{Name: "Torches", Quantity: 3, Location: "stowed"}, 1},
		{"6 torches = 2 slots", Item{Name: "Torches", Quantity: 6, Location: "stowed"}, 2},
		{"1 torch = 1 slot", Item{Name: "Torches", Quantity: 1, Location: "stowed"}, 1},
		{"10 candles = 1 slot", Item{Name: "Candles", Quantity: 10, Location: "stowed"}, 1},
		{"15 candles = 2 slots", Item{Name: "Candles", Quantity: 15, Location: "stowed"}, 2},
		{"20 arrows = 1 slot", Item{Name: "Arrows", Quantity: 20, Location: "stowed"}, 1},
		{"40 arrows = 2 slots", Item{Name: "Arrows", Quantity: 40, Location: "stowed"}, 2},
		{"12 iron spikes = 1 slot", Item{Name: "Iron Spikes", Quantity: 12, Location: "stowed"}, 1},
		{"20 quarrels = 1 slot", Item{Name: "Quarrels", Quantity: 20, Location: "stowed"}, 1},
		{"20 sling stones = 1 slot", Item{Name: "Sling Stones", Quantity: 20, Location: "stowed"}, 1},
		{"20 caltrops = 1 slot", Item{Name: "Caltrops", Quantity: 20, Location: "stowed"}, 1},
		{"10 chalk = 1 slot", Item{Name: "Chalk", Quantity: 10, Location: "stowed"}, 1},
		{"20 marbles = 1 slot", Item{Name: "Marbles", Quantity: 20, Location: "stowed"}, 1},
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

func TestIsEquippedOnCharacter(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want bool
	}{
		{"both nil", Item{Name: "Rope"}, true},
		{"in container", Item{Name: "Rope", ContainerID: new(uint(1))}, false},
		{"on companion", Item{Name: "Rope", CompanionID: new(uint(2))}, false},
		{"both set", Item{Name: "Rope", ContainerID: new(uint(1)), CompanionID: new(uint(2))}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.item.IsEquippedOnCharacter()
			if got != tc.want {
				t.Errorf("IsEquippedOnCharacter() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFindRoot(t *testing.T) {
	// Chain: item3 -> item2 -> item1 (equipped)
	items := []Item{
		{ID: 1, Name: "Backpack"},
		{ID: 2, Name: "Sack", ContainerID: new(uint(1))},
		{ID: 3, Name: "Rope", ContainerID: new(uint(2))},
	}
	byID := make(map[uint]Item)
	for _, it := range items {
		byID[it.ID] = it
	}
	root := FindRoot(items[2], byID)
	if root.ID != 1 {
		t.Errorf("FindRoot(item3) root ID = %d, want 1", root.ID)
	}
}

func TestCalculateEncumbrance(t *testing.T) {
	backpackID := uint(10)
	companionID := uint(20)
	items := []Item{
		// Equipped on character
		{ID: 10, Name: "Backpack", Quantity: 1},   // equipped container: 0 slots
		{ID: 11, Name: "Plate mail", Quantity: 1}, // 3 slots
		{ID: 12, Name: "Longsword", Quantity: 1},  // 1 slot
		// Stowed in backpack
		{ID: 13, Name: "Rope", Quantity: 1, ContainerID: &backpackID},    // 100cn = 1 slot
		{ID: 14, Name: "Torches", Quantity: 6, ContainerID: &backpackID}, // 2 slots (6/3 = 2 bundles)
		// On companion
		{ID: 15, Name: "Bedroll", Quantity: 1, CompanionID: &companionID},           // 70cn = 1 slot
		{ID: 16, Name: "Preserved Rations", Quantity: 3, CompanionID: &companionID}, // 3*20cn=60cn = 1 slot
	}

	equipped, stowed, companionSlots := CalculateEncumbrance(items)

	if equipped != 4 {
		t.Errorf("equipped = %d, want 4 (0 backpack + 3 plate + 1 longsword)", equipped)
	}
	if stowed != 3 {
		t.Errorf("stowed = %d, want 3 (1 rope + 2 torches)", stowed)
	}
	if companionSlots[companionID] != 2 {
		t.Errorf("companion %d slots = %d, want 2 (1 bedroll + 1 rations)", companionID, companionSlots[companionID])
	}
}

func TestCompanionItemsDontAffectSpeed(t *testing.T) {
	companionID := uint(1)

	// Character with light equipment (3 equipped slots → speed 40)
	// and many heavy items on companion
	items := []Item{
		{ID: 1, Name: "Longsword", Quantity: 1}, // 1 equipped slot
		{ID: 2, Name: "Shield", Quantity: 1},    // 1 equipped slot
		{ID: 3, Name: "Leather", Quantity: 1},   // 1 equipped slot
		// Companion has tons of stuff
		{ID: 10, Name: "Plate mail", Quantity: 1, CompanionID: &companionID},         // 3 slots
		{ID: 11, Name: "Preserved Rations", Quantity: 20, CompanionID: &companionID}, // 4 slots
		{ID: 12, Name: "Rope", Quantity: 5, CompanionID: &companionID},               // 5 slots
		{ID: 13, Name: "Bedroll", Quantity: 3, CompanionID: &companionID},            // 3 slots
	}

	equipped, stowed, companionSlots := CalculateEncumbrance(items)

	// Character: 3 equipped, 0 stowed → speed should be 40
	if equipped != 3 {
		t.Errorf("equipped = %d, want 3", equipped)
	}
	if stowed != 0 {
		t.Errorf("stowed = %d, want 0", stowed)
	}

	// Companion items should NOT be in equipped or stowed
	if companionSlots[companionID] == 0 {
		t.Errorf("companion slots = 0, expected items on companion")
	}

	speed := SpeedFromSlots(equipped, stowed)
	if speed != 40 {
		t.Errorf("speed = %d, want 40 (companion items should not affect character speed)", speed)
	}
}

func TestCoinItemSlots(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want int
	}{
		// Consolidated "Coins" item — qty is total coin count
		{"80 coins = 1 slot", Item{Name: "Coins", Quantity: 80}, 1},
		{"100 coins = 1 slot", Item{Name: "Coins", Quantity: 100}, 1},
		{"101 coins = 2 slots", Item{Name: "Coins", Quantity: 101}, 2},
		{"200 coins = 2 slots", Item{Name: "Coins", Quantity: 200}, 2},
		// Legacy per-denomination names still work
		{"100 gold pieces = 1 slot", Item{Name: "Gold Pieces", Quantity: 100}, 1},
		{"200 gold pieces = 2 slots", Item{Name: "Gold Pieces", Quantity: 200}, 2},
		{"50 silver pieces = 1 slot", Item{Name: "Silver Pieces", Quantity: 50}, 1},
		{"150 copper pieces = 2 slots", Item{Name: "Copper Pieces", Quantity: 150}, 2},
		{"1 platinum piece = 1 slot", Item{Name: "Platinum Pieces", Quantity: 1}, 1},
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

func TestCompanionContainerItemsDontAffectSpeed(t *testing.T) {
	companionID := uint(1)
	chestID := uint(10)

	// Character lightly equipped, companion has a chest full of stuff
	items := []Item{
		{ID: 1, Name: "Longsword", Quantity: 1}, // 1 equipped slot
		// Chest on companion
		{ID: 10, Name: "Chest (wooden, large)", Quantity: 1, CompanionID: &companionID},
		// Items in the chest (on companion)
		{ID: 11, Name: "Rope", Quantity: 1, ContainerID: &chestID},
		{ID: 12, Name: "Preserved Rations", Quantity: 10, ContainerID: &chestID},
		{ID: 13, Name: "Torches", Quantity: 6, ContainerID: &chestID},
	}

	equipped, stowed, companionSlots := CalculateEncumbrance(items)

	if equipped != 1 {
		t.Errorf("equipped = %d, want 1 (only longsword)", equipped)
	}
	if stowed != 0 {
		t.Errorf("stowed = %d, want 0 (all items are on companion)", stowed)
	}
	if companionSlots[companionID] == 0 {
		t.Error("companion slots = 0, expected items on companion")
	}

	speed := SpeedFromSlots(equipped, stowed)
	if speed != 40 {
		t.Errorf("speed = %d, want 40 (companion items should not affect character speed)", speed)
	}
}
