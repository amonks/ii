package engine

import (
	"slices"
	"testing"
)

func TestBreedStats(t *testing.T) {
	tests := []struct {
		breed      string
		wantAC     int
		wantHP     int
		wantSpeed  int
		wantLoad   int
		wantLevel  int
		wantSaves  SaveTargets
		wantAtk    string
		wantMorale int
	}{
		{"Charger", 12, 13, 40, 40, 3, SaveTargets{11, 12, 13, 14, 15}, "2 hooves (+2, 1d6)", 9},
		{"Dapple-doff", 12, 13, 30, 50, 3, SaveTargets{11, 12, 13, 14, 15}, "None", 5},
		{"Hop-clopper", 12, 13, 30, 50, 3, SaveTargets{11, 12, 13, 14, 15}, "2 hooves (+2, 1d4)", 7},
		{"Mule", 12, 9, 40, 25, 2, SaveTargets{12, 13, 14, 15, 16}, "Kick (+1, 1d4) or bite (+1, 1d3)", 8},
		{"Prigwort prancer", 12, 9, 80, 30, 2, SaveTargets{12, 13, 14, 15, 16}, "2 hooves (+1, 1d4)", 7},
		{"Yellow-flank", 12, 13, 60, 35, 3, SaveTargets{11, 12, 13, 14, 15}, "2 hooves (+2, 1d4)", 7},
		{"Townsfolk", 10, 2, 40, 10, 1, SaveTargets{12, 13, 14, 15, 16}, "Weapon (-1)", 6},
	}
	for _, tt := range tests {
		t.Run(tt.breed, func(t *testing.T) {
			stats, ok := BreedStats(tt.breed)
			if !ok {
				t.Fatalf("BreedStats(%q) not found", tt.breed)
			}
			if stats.AC != tt.wantAC {
				t.Errorf("AC = %d, want %d", stats.AC, tt.wantAC)
			}
			if stats.HPMax != tt.wantHP {
				t.Errorf("HPMax = %d, want %d", stats.HPMax, tt.wantHP)
			}
			if stats.Speed != tt.wantSpeed {
				t.Errorf("Speed = %d, want %d", stats.Speed, tt.wantSpeed)
			}
			if stats.LoadCapacity != tt.wantLoad {
				t.Errorf("LoadCapacity = %d, want %d", stats.LoadCapacity, tt.wantLoad)
			}
			if stats.Level != tt.wantLevel {
				t.Errorf("Level = %d, want %d", stats.Level, tt.wantLevel)
			}
			if stats.Saves != tt.wantSaves {
				t.Errorf("Saves = %+v, want %+v", stats.Saves, tt.wantSaves)
			}
			if stats.Attack != tt.wantAtk {
				t.Errorf("Attack = %q, want %q", stats.Attack, tt.wantAtk)
			}
			if stats.Morale != tt.wantMorale {
				t.Errorf("Morale = %d, want %d", stats.Morale, tt.wantMorale)
			}
		})
	}
}

func TestBreedStatsUnknown(t *testing.T) {
	_, ok := BreedStats("Unknown Horse")
	if ok {
		t.Error("expected unknown breed to return false")
	}
}

func TestBreedNames(t *testing.T) {
	names := BreedNames()
	if len(names) != 8 {
		t.Errorf("got %d breeds, want 8", len(names))
	}
	// Townsfolk should be in the list
	found := slices.Contains(names, "Townsfolk")
	if !found {
		t.Error("BreedNames() should contain Townsfolk")
	}
	if !slices.Contains(names, "Animal Companion") {
		t.Error("BreedNames() should contain Animal Companion")
	}
}

func TestCompanionAC(t *testing.T) {
	cases := []struct {
		name       string
		baseAC     int
		hasBarding bool
		want       int
	}{
		{"no barding", 12, false, 12},
		{"with barding", 12, true, 14},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CompanionAC(tc.baseAC, tc.hasBarding)
			if got != tc.want {
				t.Errorf("CompanionAC(%d, %v) = %d, want %d", tc.baseAC, tc.hasBarding, got, tc.want)
			}
		})
	}
}

func TestIsCompanionBreed(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"Animal Companion", true},
		{"Mule", true},
		{"Charger", true},
		{"Dapple-doff", true},
		{"mule", true},   // case insensitive
		{"CHARGER", true}, // case insensitive
		{"Longsword", false},
		{"Backpack", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsCompanionBreed(tc.name)
			if got != tc.want {
				t.Errorf("IsCompanionBreed(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestIsCompanionGear(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"Pack saddle and bridle", true},
		{"Riding saddle and bridle", true},
		{"Horse barding", true},
		{"pack saddle and bridle", true},   // case insensitive
		{"HORSE BARDING", true},            // case insensitive
		{"Riding saddle bags", false},      // saddle bags are NOT gear (they're a regular item)
		{"Longsword", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsCompanionGear(tc.name)
			if got != tc.want {
				t.Errorf("IsCompanionGear(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestCompanionSaddleTypeFromItems(t *testing.T) {
	cases := []struct {
		name  string
		items []Item
		want  string
	}{
		{"no items", nil, ""},
		{"pack saddle", []Item{{Name: "Pack saddle and bridle", Quantity: 1}}, "pack"},
		{"riding saddle", []Item{{Name: "Riding saddle and bridle", Quantity: 1}}, "riding"},
		{"barding only", []Item{{Name: "Horse barding", Quantity: 1}}, ""},
		{"mixed gear", []Item{
			{Name: "Riding saddle and bridle", Quantity: 1},
			{Name: "Horse barding", Quantity: 1},
		}, "riding"},
		{"case insensitive", []Item{{Name: "PACK SADDLE AND BRIDLE", Quantity: 1}}, "pack"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CompanionSaddleTypeFromItems(tc.items)
			if got != tc.want {
				t.Errorf("CompanionSaddleTypeFromItems() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompanionHasBardingFromItems(t *testing.T) {
	cases := []struct {
		name  string
		items []Item
		want  bool
	}{
		{"no items", nil, false},
		{"has barding", []Item{{Name: "Horse barding", Quantity: 1}}, true},
		{"no barding", []Item{{Name: "Pack saddle and bridle", Quantity: 1}}, false},
		{"case insensitive", []Item{{Name: "HORSE BARDING", Quantity: 1}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CompanionHasBardingFromItems(tc.items)
			if got != tc.want {
				t.Errorf("CompanionHasBardingFromItems() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCompanionLoadCapacity(t *testing.T) {
	cases := []struct {
		name          string
		breedCapacity int
		saddleType    string
		want          int
	}{
		{"no saddle", 25, "", 0},
		{"riding saddle", 25, "riding", 5},
		{"pack saddle mule", 25, "pack", 25},
		{"pack saddle dapple-doff", 50, "pack", 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CompanionLoadCapacity(tc.breedCapacity, tc.saddleType)
			if got != tc.want {
				t.Errorf("CompanionLoadCapacity(%d, %q) = %d, want %d", tc.breedCapacity, tc.saddleType, got, tc.want)
			}
		})
	}
}

func TestIsRetainer(t *testing.T) {
	cases := []struct {
		breed string
		want  bool
	}{
		{"Townsfolk", true},
		{"Animal Companion", false},
		{"Mule", false},
		{"Charger", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.breed, func(t *testing.T) {
			got := IsRetainer(tc.breed)
			if got != tc.want {
				t.Errorf("IsRetainer(%q) = %v, want %v", tc.breed, got, tc.want)
			}
		})
	}
}

func TestRetainerLoyalty(t *testing.T) {
	cases := []struct {
		name   string
		chaMod int
		want   int
	}{
		{"neutral CHA", 0, 7},
		{"high CHA", 2, 9},
		{"low CHA", -1, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RetainerLoyalty(tc.chaMod)
			if got != tc.want {
				t.Errorf("RetainerLoyalty(%d) = %d, want %d", tc.chaMod, got, tc.want)
			}
		})
	}
}

func TestNeedsSaddle(t *testing.T) {
	// Horse breeds need saddles
	for _, breed := range []string{"Charger", "Dapple-doff", "Hop-clopper", "Mule", "Prigwort prancer", "Yellow-flank"} {
		stats, ok := BreedStats(breed)
		if !ok {
			t.Fatalf("breed %q not found", breed)
		}
		if !stats.NeedsSaddle {
			t.Errorf("breed %q: NeedsSaddle = false, want true", breed)
		}
	}
	// Townsfolk don't need saddles
	stats, ok := BreedStats("Townsfolk")
	if !ok {
		t.Fatal("Townsfolk breed not found")
	}
	if stats.NeedsSaddle {
		t.Error("Townsfolk: NeedsSaddle = true, want false")
	}
}
