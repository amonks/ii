package engine

import (
	"fmt"
	"testing"
)

func TestThiefBackstabStats(t *testing.T) {
	if got := ThiefBackstabDamage(); got != "3d4" {
		t.Errorf("ThiefBackstabDamage() = %q, want %q", got, "3d4")
	}
	if got := ThiefBackstabBonus(); got != 4 {
		t.Errorf("ThiefBackstabBonus() = %d, want 4", got)
	}
}

func TestThiefSkillTargets(t *testing.T) {
	cases := []struct {
		level int
		want  SkillTargets
	}{
		{
			level: 1,
			want: SkillTargets{
				"Climb Wall":   4,
				"Decipher Doc.": 6,
				"Disarm Mech.":  6,
				"Legerdemain":  6,
				"Listen":        6,
				"Pick Lock":     5,
				"Search":        6,
				"Stealth":       5,
			},
		},
		{
			level: 8,
			want: SkillTargets{
				"Climb Wall":   2,
				"Decipher Doc.": 4,
				"Disarm Mech.":  4,
				"Legerdemain":  4,
				"Listen":        4,
				"Pick Lock":     4,
				"Search":        4,
				"Stealth":       4,
			},
		},
		{
			level: 15,
			want: SkillTargets{
				"Climb Wall":   2,
				"Decipher Doc.": 2,
				"Disarm Mech.":  2,
				"Legerdemain":  2,
				"Listen":        2,
				"Pick Lock":     2,
				"Search":        2,
				"Stealth":       2,
			},
		},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("level-%d", tc.level), func(t *testing.T) {
			got := ThiefSkillTargets(tc.level)
			if len(got) != len(tc.want) {
				t.Fatalf("ThiefSkillTargets(%d) length = %d, want %d", tc.level, len(got), len(tc.want))
			}
			for name, want := range tc.want {
				if got[name] != want {
					t.Errorf("ThiefSkillTargets(%d)[%q] = %d, want %d", tc.level, name, got[name], want)
				}
			}
		})
	}
}

func TestThiefSkillTargetsInvalidLevel(t *testing.T) {
	if got := ThiefSkillTargets(0); got != nil {
		t.Errorf("ThiefSkillTargets(0) = %v, want nil", got)
	}
}
