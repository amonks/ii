package engine

import "reflect"
import "testing"

func TestTurnUndeadTableCleric(t *testing.T) {
	cases := []struct {
		level int
		want  []TurnUndeadEntry
	}{
		{
			level: 1,
			want: []TurnUndeadEntry{
				{UndeadHD: "1", Target: "7"},
				{UndeadHD: "2", Target: "9"},
				{UndeadHD: "3", Target: "11"},
				{UndeadHD: "4", Target: "-"},
				{UndeadHD: "5", Target: "-"},
				{UndeadHD: "6", Target: "-"},
				{UndeadHD: "7", Target: "-"},
				{UndeadHD: "8", Target: "-"},
				{UndeadHD: "9", Target: "-"},
				{UndeadHD: "10", Target: "-"},
				{UndeadHD: "11", Target: "-"},
			},
		},
		{
			level: 9,
			want: []TurnUndeadEntry{
				{UndeadHD: "1", Target: "T"},
				{UndeadHD: "2", Target: "T"},
				{UndeadHD: "3", Target: "3"},
				{UndeadHD: "4", Target: "5"},
				{UndeadHD: "5", Target: "7"},
				{UndeadHD: "6", Target: "9"},
				{UndeadHD: "7", Target: "11"},
				{UndeadHD: "8", Target: "13"},
				{UndeadHD: "9", Target: "15"},
				{UndeadHD: "10", Target: "17"},
				{UndeadHD: "11", Target: "19"},
			},
		},
		{
			level: 15,
			want: []TurnUndeadEntry{
				{UndeadHD: "1", Target: "D"},
				{UndeadHD: "2", Target: "D"},
				{UndeadHD: "3", Target: "D"},
				{UndeadHD: "4", Target: "D"},
				{UndeadHD: "5", Target: "T"},
				{UndeadHD: "6", Target: "T"},
				{UndeadHD: "7", Target: "11"},
				{UndeadHD: "8", Target: "13"},
				{UndeadHD: "9", Target: "15"},
				{UndeadHD: "10", Target: "17"},
				{UndeadHD: "11", Target: "19"},
			},
		},
	}

	for _, tc := range cases {
		t.Run("level", func(t *testing.T) {
			got := TurnUndeadTable("Cleric", tc.level)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("TurnUndeadTable level %d = %v, want %v", tc.level, got, tc.want)
			}
		})
	}
}

func TestTurnUndeadTableFriar(t *testing.T) {
	got := TurnUndeadTable("Friar", 4)
	if len(got) == 0 {
		t.Fatalf("TurnUndeadTable(Friar, 4) returned empty table")
	}
	if got[3].Target != "10" {
		t.Errorf("TurnUndeadTable(Friar, 4)[3].Target = %q, want 10", got[3].Target)
	}
}

func TestTurnUndeadTableInvalidInput(t *testing.T) {
	if got := TurnUndeadTable("Bard", 1); got != nil {
		t.Errorf("TurnUndeadTable(Bard, 1) = %v, want nil", got)
	}
	if got := TurnUndeadTable("Cleric", 0); got != nil {
		t.Errorf("TurnUndeadTable(Cleric, 0) = %v, want nil", got)
	}
}
