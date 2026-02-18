package engine

import "testing"

func TestKnightLevelForXP(t *testing.T) {
	cases := []struct {
		xp   int
		want int
	}{
		{0, 1},
		{2249, 1},
		{2250, 2},
		{4499, 2},
		{4500, 3},
		{9000, 4},
		{18000, 5},
		{1070000, 15},
	}
	for _, tc := range cases {
		got := KnightLevelForXP(tc.xp)
		if got != tc.want {
			t.Errorf("KnightLevelForXP(%d) = %d, want %d", tc.xp, got, tc.want)
		}
	}
}

func TestKnightAttackBonus(t *testing.T) {
	cases := []struct {
		level int
		want  int
	}{
		{1, 1},
		{3, 2},
		{4, 3},
		{6, 4},
		{7, 5},
		{9, 6},
		{10, 7},
		{12, 8},
		{13, 9},
		{15, 10},
	}
	for _, tc := range cases {
		got := KnightAttackBonus(tc.level)
		if got != tc.want {
			t.Errorf("KnightAttackBonus(%d) = %d, want %d", tc.level, got, tc.want)
		}
	}
}

func TestKnightSaveTargets(t *testing.T) {
	cases := []struct {
		level int
		want  SaveTargets
	}{
		{1, SaveTargets{Death: 12, Wands: 13, Paralysis: 12, Breath: 15, Spells: 15}},
		{4, SaveTargets{Death: 10, Wands: 11, Paralysis: 10, Breath: 13, Spells: 13}},
	}
	for _, tc := range cases {
		got := KnightSaveTargets(tc.level)
		if got != tc.want {
			t.Errorf("KnightSaveTargets(%d) = %+v, want %+v", tc.level, got, tc.want)
		}
	}
}

func TestKnightTraits(t *testing.T) {
	t.Run("MonsterSlayer", func(t *testing.T) {
		traits := KnightTraits(4)
		if traits.MonsterSlayer {
			t.Error("level 4 should not have MonsterSlayer")
		}
		traits = KnightTraits(5)
		if !traits.MonsterSlayer {
			t.Error("level 5 should have MonsterSlayer")
		}
	})

	t.Run("Knighthood", func(t *testing.T) {
		traits := KnightTraits(2)
		if traits.Knighthood {
			t.Error("level 2 should not have Knighthood")
		}
		traits = KnightTraits(3)
		if !traits.Knighthood {
			t.Error("level 3 should have Knighthood")
		}
	})
}
