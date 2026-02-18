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
