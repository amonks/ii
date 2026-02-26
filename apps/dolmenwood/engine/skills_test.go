package engine

import (
	"reflect"
	"testing"
)

func TestBardSkillTargets(t *testing.T) {
	cases := []struct {
		level int
		want  SkillTargets
	}{
		{
			level: 1,
			want: SkillTargets{
				"Decipher Doc.": 6,
				"Legerdemain":   6,
				"Listen":        5,
				"Monster Lore":  5,
			},
		},
		{
			level: 8,
			want: SkillTargets{
				"Decipher Doc.": 4,
				"Legerdemain":   4,
				"Listen":        4,
				"Monster Lore":  3,
			},
		},
		{
			level: 15,
			want: SkillTargets{
				"Decipher Doc.": 2,
				"Legerdemain":   2,
				"Listen":        2,
				"Monster Lore":  2,
			},
		},
	}

	for _, tc := range cases {
		t.Run("level", func(t *testing.T) {
			got := BardSkillTargets(tc.level)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("BardSkillTargets(%d) = %v, want %v", tc.level, got, tc.want)
			}
		})
	}
}

func TestBardSkillTargetsInvalidLevel(t *testing.T) {
	if got := BardSkillTargets(0); got != nil {
		t.Errorf("BardSkillTargets(0) = %v, want nil", got)
	}
}

func TestBardSkillNames(t *testing.T) {
	want := []string{"Decipher Doc.", "Legerdemain", "Listen", "Monster Lore"}
	if got := BardSkillNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("BardSkillNames() = %v, want %v", got, want)
	}
}

func TestHunterSkillTargets(t *testing.T) {
	cases := []struct {
		level int
		want  SkillTargets
	}{
		{
			level: 1,
			want: SkillTargets{
				"Alertness": 6,
				"Stalking":  6,
				"Survival":  5,
				"Tracking":  5,
			},
		},
		{
			level: 8,
			want: SkillTargets{
				"Alertness": 5,
				"Stalking":  4,
				"Survival":  3,
				"Tracking":  3,
			},
		},
		{
			level: 15,
			want: SkillTargets{
				"Alertness": 2,
				"Stalking":  2,
				"Survival":  2,
				"Tracking":  2,
			},
		},
	}

	for _, tc := range cases {
		t.Run("level", func(t *testing.T) {
			got := HunterSkillTargets(tc.level)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("HunterSkillTargets(%d) = %v, want %v", tc.level, got, tc.want)
			}
		})
	}
}

func TestHunterSkillTargetsInvalidLevel(t *testing.T) {
	if got := HunterSkillTargets(0); got != nil {
		t.Errorf("HunterSkillTargets(0) = %v, want nil", got)
	}
}

func TestHunterSkillNames(t *testing.T) {
	want := []string{"Alertness", "Stalking", "Survival", "Tracking"}
	if got := HunterSkillNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("HunterSkillNames() = %v, want %v", got, want)
	}
}
