package engine

import "testing"

func TestBreggleGazeUses(t *testing.T) {
	cases := []struct {
		level int
		want  int
	}{
		{1, 0},
		{3, 0},
		{4, 1},
		{5, 1},
		{6, 2},
		{7, 2},
		{8, 3},
		{9, 3},
		{10, 4},
		{12, 4},
	}

	for _, tc := range cases {
		if got := BreggleGazeUses(tc.level); got != tc.want {
			t.Errorf("BreggleGazeUses(%d) = %d, want %d", tc.level, got, tc.want)
		}
	}
}

func TestBreggleHornDamage(t *testing.T) {
	cases := []struct {
		level int
		want  string
	}{
		{1, "1d4"},
		{2, "1d4"},
		{3, "1d4+1"},
		{5, "1d4+1"},
		{6, "1d6"},
		{8, "1d6"},
		{9, "1d6+1"},
		{10, "1d6+2"},
		{12, "1d6+2"},
	}

	for _, tc := range cases {
		if got := BreggleHornDamage(tc.level); got != tc.want {
			t.Errorf("BreggleHornDamage(%d) = %q, want %q", tc.level, got, tc.want)
		}
	}
}
