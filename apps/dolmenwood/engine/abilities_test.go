package engine

import "testing"

func TestModifier(t *testing.T) {
	cases := []struct {
		score int
		want  int
	}{
		{3, -3},
		{4, -2},
		{5, -2},
		{6, -1},
		{8, -1},
		{9, 0},
		{12, 0},
		{13, +1},
		{15, +1},
		{16, +2},
		{17, +2},
		{18, +3},
	}
	for _, tc := range cases {
		got := Modifier(tc.score)
		if got != tc.want {
			t.Errorf("Modifier(%d) = %d, want %d", tc.score, got, tc.want)
		}
	}
}

func TestPrimeAbilityXPModifier(t *testing.T) {
	t.Run("single prime", func(t *testing.T) {
		cases := []struct {
			score int
			want  int
		}{
			{3, -20},
			{5, -20},
			{6, -10},
			{8, -10},
			{9, 0},
			{13, 5},
			{15, 5},
			{16, 10},
			{18, 10},
		}
		for _, tc := range cases {
			scores := map[string]int{"str": tc.score}
			primes := []string{"str"}
			got := PrimeAbilityXPModifier(scores, primes)
			if got != tc.want {
				t.Errorf("PrimeAbilityXPModifier(score=%d) = %d%%, want %d%%", tc.score, got, tc.want)
			}
		}
	})

	t.Run("multiple primes takes lowest", func(t *testing.T) {
		scores := map[string]int{"str": 16, "wis": 6}
		primes := []string{"str", "wis"}
		got := PrimeAbilityXPModifier(scores, primes)
		if got != -10 {
			t.Errorf("PrimeAbilityXPModifier(str=16,wis=6) = %d%%, want -10%%", got)
		}
	})
}

func TestACFromArmor(t *testing.T) {
	cases := []struct {
		name     string
		baseAC   int
		dexScore int
		shield   bool
		want     int
	}{
		{"unarmoured + DEX 14", 10, 14, false, 11},
		{"plate + DEX 8", 16, 8, false, 15},
		{"plate + shield + DEX 10", 16, 10, true, 17},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ACFromArmor(tc.baseAC, tc.dexScore, tc.shield)
			if got != tc.want {
				t.Errorf("ACFromArmor(%d, %d, %v) = %d, want %d", tc.baseAC, tc.dexScore, tc.shield, got, tc.want)
			}
		})
	}
}
