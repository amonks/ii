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
		{"Backpack", 10, true},
		{"Sack", 10, true},
		{"Belt Pouch", 1, true},
		{"backpack", 10, true},  // case insensitive
		{"Longsword", 0, false}, // not a container
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
