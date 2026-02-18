package engine

import "testing"

func TestBreedStats(t *testing.T) {
	tests := []struct {
		breed     string
		wantAC    int
		wantHP    int
		wantSpeed int
		wantLoad  int
		wantLevel int
		wantSaves SaveTargets
		wantAtk   string
		wantMorale int
	}{
		{"Charger", 12, 13, 40, 40, 3, SaveTargets{11, 12, 13, 14, 15}, "2 hooves (+2, 1d6)", 9},
		{"Dapple-doff", 12, 13, 30, 50, 3, SaveTargets{11, 12, 13, 14, 15}, "None", 5},
		{"Hop-clopper", 12, 13, 30, 50, 3, SaveTargets{11, 12, 13, 14, 15}, "2 hooves (+2, 1d4)", 7},
		{"Mule", 12, 9, 40, 25, 2, SaveTargets{12, 13, 14, 15, 16}, "Kick (+1, 1d4) or bite (+1, 1d3)", 8},
		{"Prigwort prancer", 12, 9, 80, 30, 2, SaveTargets{12, 13, 14, 15, 16}, "2 hooves (+1, 1d4)", 7},
		{"Yellow-flank", 12, 13, 60, 35, 3, SaveTargets{11, 12, 13, 14, 15}, "2 hooves (+2, 1d4)", 7},
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
	if len(names) != 6 {
		t.Errorf("got %d breeds, want 6", len(names))
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
