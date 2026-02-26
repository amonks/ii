package engine

import "testing"

func TestClassSpellSlots(t *testing.T) {
	cases := []struct {
		name  string
		class string
		level int
		want  *SpellSlots
	}{
		{
			name:  "non spellcaster",
			class: "Fighter",
			level: 1,
			want:  nil,
		},
		{
			name:  "cleric level 1",
			class: "Cleric",
			level: 1,
			want:  &SpellSlots{},
		},
		{
			name:  "cleric level 4",
			class: "Cleric",
			level: 4,
			want:  &SpellSlots{Level1: 2, Level2: 1},
		},
		{
			name:  "friar level 7",
			class: "Friar",
			level: 7,
			want:  &SpellSlots{Level1: 3, Level2: 3, Level3: 2, Level4: 1},
		},
		{
			name:  "magician level 11",
			class: "Magician",
			level: 11,
			want:  &SpellSlots{Level1: 4, Level2: 3, Level3: 3, Level4: 2, Level5: 2, Level6: 1},
		},
		{
			name:  "invalid level",
			class: "Magician",
			level: 0,
			want:  nil,
		},
		{
			name:  "too high level",
			class: "Magician",
			level: 16,
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassSpellSlots(tc.class, tc.level)
			if (got == nil) != (tc.want == nil) {
				t.Fatalf("ClassSpellSlots(%q, %d) = %v, want %v", tc.class, tc.level, got, tc.want)
			}
			if got == nil {
				return
			}
			if *got != *tc.want {
				t.Fatalf("ClassSpellSlots(%q, %d) = %+v, want %+v", tc.class, tc.level, *got, *tc.want)
			}
		})
	}
}
