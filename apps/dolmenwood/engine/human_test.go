package engine

import "testing"

func TestHumanTotalXPModifier(t *testing.T) {
	cases := []struct {
		name       string
		primeScore int
		want       int
	}{
		{"high prime", 15, 15},   // +5% prime + 10% human = +15%
		{"low prime", 8, 0},      // -10% prime + 10% human = 0%
		{"very low prime", 3, -10}, // -20% prime + 10% human = -10%
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scores := map[string]int{"str": tc.primeScore}
			primes := []string{"str"}
			got := HumanTotalXPModifier(scores, primes)
			if got != tc.want {
				t.Errorf("HumanTotalXPModifier(prime=%d) = %d%%, want %d%%", tc.primeScore, got, tc.want)
			}
		})
	}
}
