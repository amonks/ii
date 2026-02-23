package engine

import "testing"

func TestWeaponStats(t *testing.T) {
	tests := []struct {
		name       string
		wantDamage string
		wantFound  bool
	}{
		{"Longsword", "1d8", true},
		{"Dagger", "1d4", true},
		{"Crossbow", "1d8", true},
		{"Spear", "1d6", true},
		{"longsword", "1d8", true},    // case insensitive
		{"LONGSWORD", "1d8", true},    // case insensitive
		{"Magic Wand", "", false},     // unknown
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats, ok := WeaponStats(tt.name)
			if ok != tt.wantFound {
				t.Fatalf("WeaponStats(%q) found = %v, want %v", tt.name, ok, tt.wantFound)
			}
			if !ok {
				return
			}
			if stats.Damage != tt.wantDamage {
				t.Errorf("Damage = %q, want %q", stats.Damage, tt.wantDamage)
			}
		})
	}
}

func TestItemWeight(t *testing.T) {
	tests := []struct {
		name       string
		wantWeight int
		wantFound  bool
	}{
		{"Longsword", 30, true},        // weapon
		{"Shield", 100, true},          // armor
		{"Preserved Rations", 20, true}, // general item
		{"Rope", 100, true},            // general item
		{"Magic Orb", 0, false},        // unknown
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, ok := ItemWeight(tt.name)
			if ok != tt.wantFound {
				t.Fatalf("ItemWeight(%q) found = %v, want %v", tt.name, ok, tt.wantFound)
			}
			if w != tt.wantWeight {
				t.Errorf("weight = %d, want %d", w, tt.wantWeight)
			}
		})
	}
}

func TestContainerCapacity(t *testing.T) {
	tests := []struct {
		name      string
		wantSlots int
		wantFound bool
	}{
		// Personal containers
		{"Backpack", 10, true},
		{"Sack", 10, true},
		{"Belt Pouch", 1, true},
		{"backpack", 10, true}, // case insensitive

		// Heavy containers
		{"Casket (iron, large)", 8, true},
		{"Casket (iron, small)", 3, true},
		{"Chest (wooden, large)", 10, true},
		{"Chest (wooden, small)", 3, true},
		{"Scroll Case", 1, true},
		{"Riding Saddle Bags", 5, true},

		// Vehicles
		{"Cart", 100, true},
		{"Wagon", 200, true},

		// Not containers
		{"Longsword", 0, false},
		{"Rope", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slots, ok := ContainerCapacity(tt.name)
			if ok != tt.wantFound {
				t.Fatalf("ContainerCapacity(%q) found = %v, want %v", tt.name, ok, tt.wantFound)
			}
			if slots != tt.wantSlots {
				t.Errorf("slots = %d, want %d", slots, tt.wantSlots)
			}
		})
	}
}

func TestIsPersonalContainer(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Backpack", true},
		{"Sack", true},
		{"Belt Pouch", true},
		{"Chest (wooden, large)", false},
		{"Casket (iron, large)", false},
		{"Scroll Case", false},
		{"Riding Saddle Bags", false},
		{"Longsword", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPersonalContainer(tt.name)
			if got != tt.want {
				t.Errorf("IsPersonalContainer(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsContainer(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Backpack", true},
		{"Sack", true},
		{"Belt Pouch", true},
		{"Chest (wooden, large)", true},
		{"Chest (wooden, small)", true},
		{"Casket (iron, large)", true},
		{"Casket (iron, small)", true},
		{"Scroll Case", true},
		{"Riding Saddle Bags", true},
		{"Longsword", false},
		{"Rope", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsContainer(tt.name)
			if got != tt.want {
				t.Errorf("IsContainer(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestArmorStats(t *testing.T) {
	tests := []struct {
		name     string
		wantAC   int
		wantFound bool
	}{
		{"Leather", 12, true},
		{"Chainmail", 14, true},
		{"Plate mail", 16, true},
		{"Shield", 1, true},
		{"leather", 12, true},       // case insensitive
		{"Fancy Hat", 0, false},     // unknown
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats, ok := ArmorStats(tt.name)
			if ok != tt.wantFound {
				t.Fatalf("ArmorStats(%q) found = %v, want %v", tt.name, ok, tt.wantFound)
			}
			if !ok {
				return
			}
			if stats.AC != tt.wantAC {
				t.Errorf("AC = %d, want %d", stats.AC, tt.wantAC)
			}
		})
	}
}

func TestACFromEquippedItems(t *testing.T) {
	cases := []struct {
		name      string
		items     []Item
		dexScore  int
		wantAC    int
		wantArmor string
	}{
		{
			name:     "unarmoured",
			items:    nil,
			dexScore: 10,
			wantAC:   10,
		},
		{
			name: "leather equipped",
			items: []Item{
				{Name: "Leather", Quantity: 1, Location: "equipped"},
			},
			dexScore: 10,
			wantAC:   12,
			wantArmor: "Leather",
		},
		{
			name: "chainmail with dex bonus",
			items: []Item{
				{Name: "Chainmail", Quantity: 1, Location: "equipped"},
			},
			dexScore: 14,
			wantAC:   15,
			wantArmor: "Chainmail",
		},
		{
			name: "plate mail with shield",
			items: []Item{
				{Name: "Plate mail", Quantity: 1, Location: "equipped"},
				{Name: "Shield", Quantity: 1, Location: "equipped"},
			},
			dexScore: 10,
			wantAC:   17,
			wantArmor: "Plate mail",
		},
		{
			name: "shield only",
			items: []Item{
				{Name: "Shield", Quantity: 1, Location: "equipped"},
			},
			dexScore: 10,
			wantAC:   11,
		},
		{
			name: "armor in stowed is ignored",
			items: []Item{
				{Name: "Chainmail", Quantity: 1, Location: "stowed"},
			},
			dexScore: 10,
			wantAC:   10,
		},
		{
			name: "dex penalty",
			items: []Item{
				{Name: "Plate mail", Quantity: 1, Location: "equipped"},
			},
			dexScore: 6,
			wantAC:   15,
			wantArmor: "Plate mail",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ac, armorName := ACFromEquippedItems(tc.items, tc.dexScore)
			if ac != tc.wantAC {
				t.Errorf("AC = %d, want %d", ac, tc.wantAC)
			}
			if tc.wantArmor == "" {
				if armorName != "" {
					t.Errorf("armor = %q, want %q", armorName, "")
				}
				return
			}
			if armorName != tc.wantArmor {
				t.Errorf("armor = %q, want %q", armorName, tc.wantArmor)
			}
		})
	}
}

func TestCharacterACBreggleFur(t *testing.T) {
	cases := []struct {
		name      string
		kindred   string
		items     []Item
		dexScore  int
		wantAC    int
		wantArmor string
	}{
		{
			name:     "breggle unarmoured",
			kindred:  "Breggle",
			items:    nil,
			dexScore: 10,
			wantAC:   11,
		},
		{
			name:    "breggle leather armour",
			kindred: "Breggle",
			items: []Item{
				{Name: "Leather", Quantity: 1, Location: "equipped"},
			},
			dexScore:  10,
			wantAC:    13,
			wantArmor: "Leather",
		},
		{
			name:    "breggle chainmail",
			kindred: "Breggle",
			items: []Item{
				{Name: "Chainmail", Quantity: 1, Location: "equipped"},
			},
			dexScore:  10,
			wantAC:    14,
			wantArmor: "Chainmail",
		},
		{
			name:     "human unarmoured",
			kindred:  "Human",
			items:    nil,
			dexScore: 10,
			wantAC:   10,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ac, armor := CharacterAC(tc.kindred, tc.items, tc.dexScore)
			if ac != tc.wantAC {
				t.Errorf("AC = %d, want %d", ac, tc.wantAC)
			}
			if tc.wantArmor == "" {
				if armor != "" {
					t.Errorf("armor = %q, want %q", armor, "")
				}
				return
			}
			if armor != tc.wantArmor {
				t.Errorf("armor = %q, want %q", armor, tc.wantArmor)
			}
		})
	}
}

func TestArmorContributors(t *testing.T) {
	cases := []struct {
		name       string
		items      []Item
		wantArmor  string
		wantShield bool
	}{
		{
			name: "armor and shield",
			items: []Item{
				{Name: "Chainmail", Quantity: 1, Location: "equipped"},
				{Name: "Shield", Quantity: 1, Location: "equipped"},
			},
			wantArmor:  "Chainmail",
			wantShield: true,
		},
		{
			name: "shield only",
			items: []Item{
				{Name: "Shield", Quantity: 1, Location: "equipped"},
			},
			wantShield: true,
		},
		{
			name: "no armor",
			items: []Item{
				{Name: "Rope", Quantity: 1, Location: "equipped"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			armor, hasShield := ArmorContributors(tc.items)
			if armor != tc.wantArmor {
				t.Errorf("armor = %q, want %q", armor, tc.wantArmor)
			}
			if hasShield != tc.wantShield {
				t.Errorf("hasShield = %v, want %v", hasShield, tc.wantShield)
			}
		})
	}
}

func TestEquippedWeapons(t *testing.T) {
	items := []Item{
		{Name: "Longsword", Quantity: 1, Location: "equipped"},
		{Name: "Dagger", Quantity: 1, Location: "equipped"},
		{Name: "Rope", Quantity: 1, Location: "equipped"},
		{Name: "Shortbow", Quantity: 1, Location: "stowed"},
	}
	weapons := EquippedWeapons(items)
	if len(weapons) != 2 {
		t.Fatalf("got %d weapons, want 2", len(weapons))
	}
	if weapons[0].Name != "Longsword" {
		t.Errorf("weapons[0].Name = %q, want %q", weapons[0].Name, "Longsword")
	}
	if weapons[0].Damage != "1d8" {
		t.Errorf("weapons[0].Damage = %q, want %q", weapons[0].Damage, "1d8")
	}
	if weapons[1].Name != "Dagger" {
		t.Errorf("weapons[1].Name = %q, want %q", weapons[1].Name, "Dagger")
	}
}

func TestEquippedWeaponsEmpty(t *testing.T) {
	weapons := EquippedWeapons(nil)
	if len(weapons) != 0 {
		t.Errorf("got %d weapons, want 0", len(weapons))
	}
}

func TestItemSlotCost(t *testing.T) {
	tests := []struct {
		name     string
		wantSlot int
	}{
		// Armor: light=1, medium=2, heavy=3, shield=1
		{"leather", 1},
		{"bark", 1},
		{"chainmail", 2},
		{"chain mail", 2},
		{"pinecone", 2},
		{"plate mail", 3},
		{"full plate", 3},
		{"shield", 1},

		// Weapons: two-handed melee=2, others=1
		{"polearm", 2},
		{"two-handed sword", 2},
		{"staff", 2},
		{"longsword", 1},
		{"dagger", 1},
		{"crossbow", 1},
		{"shortbow", 1},
		{"sling", 1},

		// Clothing: 0 slots
		{"clothes", 0},
		{"clothes, common", 0},
		{"clothes, extravagant", 0},
		{"clothes, fine", 0},
		{"habit, friar's", 0},
		{"robes", 0},
		{"robes, ritual", 0},
		{"winter cloak", 0},

		// Tiny items: 0 slots
		{"bell", 0},
		{"holy symbol", 0},
		{"holy symbol (wooden)", 0},
		{"holy symbol (silver)", 0},
		{"holy symbol (gold)", 0},
		{"paper", 0},
		{"parchment", 0},
		{"quill", 0},
		{"whistle", 0},
		{"pipeleaf", 0},
		{"fungi", 0},
		{"herbs", 0},

		// Bulky items: 2 slots
		{"barrel", 2},
		{"casket (iron, large)", 2},
		{"casket (iron, small)", 2},
		{"chest (wooden, large)", 2},
		{"chest (wooden, small)", 2},
		{"pole", 2},
		{"sledgehammer", 2},
		{"rope ladder", 2},
		{"firewood", 2},

		// Containers: 1 slot
		{"backpack", 1},
		{"sack", 1},
		{"belt pouch", 1},

		// General items: 1 slot
		{"rope", 1},
		{"lantern", 1},
		{"crowbar", 1},
		{"bedroll", 1},
		{"preserved rations", 1},

		// Unknown: 1 slot
		{"magic orb", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ItemSlotCost(tt.name)
			if got != tt.wantSlot {
				t.Errorf("ItemSlotCost(%q) = %d, want %d", tt.name, got, tt.wantSlot)
			}
		})
	}
}

func TestArmorBulk(t *testing.T) {
	tests := []struct {
		name     string
		wantBulk int
	}{
		{"plate mail", 3},
		{"full plate", 3},
		{"chainmail", 2},
		{"chain mail", 2},
		{"pinecone", 2},
		{"leather", 1},
		{"bark", 1},
		{"shield", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, ok := ArmorStats(tt.name)
			if !ok {
				t.Fatalf("ArmorStats(%q) not found", tt.name)
			}
			if a.Bulk != tt.wantBulk {
				t.Errorf("Bulk = %d, want %d", a.Bulk, tt.wantBulk)
			}
		})
	}
}
