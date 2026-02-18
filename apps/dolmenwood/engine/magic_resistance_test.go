package engine

import "testing"

func TestMagicResistance(t *testing.T) {
	cases := []struct {
		name    string
		kindred string
		wis     int
		want    int
	}{
		{name: "human average wisdom", kindred: "Human", wis: 12, want: 0},
		{name: "elf gains bonus", kindred: "Elf", wis: 12, want: 2},
		{name: "grimalkin bonus stacks with wisdom", kindred: "GRIMALKIN", wis: 15, want: 3},
		{name: "mossling keeps wisdom modifier", kindred: "Mossling", wis: 6, want: -1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MagicResistance(tc.kindred, tc.wis)
			if got != tc.want {
				t.Errorf("MagicResistance(%q, %d) = %d, want %d", tc.kindred, tc.wis, got, tc.want)
			}
		})
	}
}
