package engine

import "testing"

func TestBreedStats(t *testing.T) {
	tests := []struct {
		breed        string
		wantAC       int
		wantHP       int
		wantSpeed    int
		wantLoad     int
	}{
		{"Charger", 12, 13, 40, 40},
		{"Dapple-doff", 12, 13, 30, 50},
		{"Hop-clopper", 12, 13, 30, 50},
		{"Mule", 12, 9, 40, 25},
		{"Prigwort prancer", 12, 9, 80, 30},
		{"Yellow-flank", 12, 13, 60, 35},
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
