package engine

import "testing"

func TestApplyXPModifiers(t *testing.T) {
	cases := []struct {
		name       string
		base       int
		modPercent int
		want       int
	}{
		{"positive modifier", 1000, 15, 1150},
		{"negative modifier", 1000, -10, 900},
		{"zero modifier", 1000, 0, 1000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyXPModifiers(tc.base, tc.modPercent)
			if got != tc.want {
				t.Errorf("ApplyXPModifiers(%d, %d) = %d, want %d", tc.base, tc.modPercent, got, tc.want)
			}
		})
	}
}

func TestDetectLevelUp(t *testing.T) {
	cases := []struct {
		name     string
		class   string
		level   int
		xp      int
		wantLvl int
		wantUp  bool
	}{
		{"level up at 2250", "Knight", 1, 2250, 2, true},
		{"no level up at 2249", "Knight", 1, 2249, 1, false},
		{"level up to 3", "Knight", 2, 4500, 3, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newLevel, leveledUp := DetectLevelUp(tc.class, tc.level, tc.xp)
			if newLevel != tc.wantLvl {
				t.Errorf("DetectLevelUp(%s, %d, %d) level = %d, want %d", tc.class, tc.level, tc.xp, newLevel, tc.wantLvl)
			}
			if leveledUp != tc.wantUp {
				t.Errorf("DetectLevelUp(%s, %d, %d) leveled = %v, want %v", tc.class, tc.level, tc.xp, leveledUp, tc.wantUp)
			}
		})
	}
}

func TestXPToNextLevel(t *testing.T) {
	cases := []struct {
		name  string
		class string
		level int
		xp    int
		want  int
	}{
		{"level 1 no xp", "Knight", 1, 0, 2250},
		{"level 1 some xp", "Knight", 1, 1000, 1250},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := XPToNextLevel(tc.class, tc.level, tc.xp)
			if got != tc.want {
				t.Errorf("XPToNextLevel(%s, %d, %d) = %d, want %d", tc.class, tc.level, tc.xp, got, tc.want)
			}
		})
	}
}
