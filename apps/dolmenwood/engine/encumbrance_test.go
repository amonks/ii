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

func TestTotalSlots(t *testing.T) {
	items := []Item{
		{SlotCost: 1, Quantity: 1, Location: "equipped"},
		{SlotCost: 2, Quantity: 1, Location: "equipped"},
		{SlotCost: 1, Quantity: 3, Location: "stowed"},
		{SlotCost: 1, Quantity: 1, Location: "companion:1"},
	}

	equipped := TotalEquippedSlots(items)
	if equipped != 3 {
		t.Errorf("TotalEquippedSlots = %d, want 3", equipped)
	}

	stowed := TotalStowedSlots(items)
	if stowed != 3 {
		t.Errorf("TotalStowedSlots = %d, want 3", stowed)
	}
}
