package engine

import "testing"

func TestConditionalSaveBonuses_KnightMossling(t *testing.T) {
	// Mossling knight should get bonuses from both kindred (Resilience) and class (Strength of Will)
	moon := &MoonSign{Moon: "Grinning moon", Phase: "Waxing", Effect: "There is a 50% chance that guardian undead will ignore your presence. (Though they act normally if you provoke them.)"}
	bonuses := ConditionalSaveBonuses("Mossling", "Knight", 1, moon)

	sources := map[string]bool{}
	for _, b := range bonuses {
		sources[b.Source] = true
	}
	if !sources["Resilience"] {
		t.Error("expected Resilience save bonus for Mossling")
	}
	if !sources["Strength of Will"] {
		t.Error("expected Strength of Will save bonus for Knight")
	}
}

func TestConditionalSaveBonuses_MoonSignWithSaveBonus(t *testing.T) {
	// Maiden's moon Full grants +2 vs charms and glamours
	moon := &MoonSign{Moon: "Maiden's moon", Phase: "Full", Effect: "+2 bonus to saving throws against charms and glamours."}
	bonuses := ConditionalSaveBonuses("Human", "Fighter", 1, moon)

	found := false
	for _, b := range bonuses {
		if b.Source == "Moon Sign" {
			found = true
			if b.Description != "+2 bonus to saving throws against charms and glamours." {
				t.Errorf("Moon Sign description = %q", b.Description)
			}
		}
	}
	if !found {
		t.Error("expected Moon Sign save bonus for Maiden's moon Full")
	}
}

func TestConditionalSaveBonuses_MoonSignWithoutSaveBonus(t *testing.T) {
	// Beast moon Waxing grants a reaction bonus, not a save bonus
	moon := &MoonSign{Moon: "Beast moon", Phase: "Waxing", Effect: "+1 reaction bonus when interacting with dogs and horses."}
	bonuses := ConditionalSaveBonuses("Human", "Fighter", 1, moon)

	for _, b := range bonuses {
		if b.Source == "Moon Sign" {
			t.Errorf("unexpected Moon Sign save bonus: %q", b.Description)
		}
	}
}

func TestConditionalSaveBonuses_NilMoonSign(t *testing.T) {
	bonuses := ConditionalSaveBonuses("Human", "Fighter", 1, nil)
	for _, b := range bonuses {
		if b.Source == "Moon Sign" {
			t.Errorf("unexpected Moon Sign save bonus with nil moon sign: %q", b.Description)
		}
	}
}

func TestConditionalSaveBonuses_HunterTrophies(t *testing.T) {
	bonuses := ConditionalSaveBonuses("Human", "Hunter", 1, nil)

	found := false
	for _, b := range bonuses {
		if b.Source == "Trophies" {
			found = true
		}
	}
	if !found {
		t.Error("expected Trophies save bonus for Hunter")
	}
}

func TestConditionalSaveBonuses_NarrowMoonPenalty(t *testing.T) {
	// Narrow moon Waxing includes a save penalty
	moon := &MoonSign{Moon: "Narrow moon", Phase: "Waxing", Effect: "+1 reaction bonus when interacting with fairies, but suffer a -1 penalty to all saving throws against fairy magic."}
	bonuses := ConditionalSaveBonuses("Human", "Fighter", 1, moon)

	found := false
	for _, b := range bonuses {
		if b.Source == "Moon Sign" {
			found = true
			if b.Description != "-1 penalty to all saving throws against fairy magic." {
				t.Errorf("Moon Sign description = %q", b.Description)
			}
		}
	}
	if !found {
		t.Error("expected Moon Sign save penalty for Narrow moon Waxing")
	}
}

func TestConditionalSaveBonuses_AllMoonSaveBonuses(t *testing.T) {
	// Verify all moon signs that should produce save bonuses
	cases := []struct {
		moon  string
		phase string
	}{
		{"Grinning moon", "Full"},
		{"Squamous moon", "Full"},
		{"Maiden's moon", "Full"},
		{"Witch's moon", "Full"},
		{"Narrow moon", "Waxing"},
		{"Black moon", "Full"},
		{"Black moon", "Waning"},
	}
	for _, tc := range cases {
		sign, ok := moonSignByMoonPhase(tc.moon, tc.phase)
		if !ok {
			t.Fatalf("no moon sign for %s %s", tc.moon, tc.phase)
		}
		bonuses := ConditionalSaveBonuses("Human", "Fighter", 1, &sign)
		found := false
		for _, b := range bonuses {
			if b.Source == "Moon Sign" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected save bonus for %s %s", tc.moon, tc.phase)
		}
	}
}

// moonSignByMoonPhase is a test helper to look up a moon sign by moon and phase.
func moonSignByMoonPhase(moon, phase string) (MoonSign, bool) {
	for _, r := range moonRanges {
		if r.Moon == moon && r.Phase == phase {
			return MoonSign{Moon: r.Moon, Phase: r.Phase, Effect: r.Effect}, true
		}
	}
	return MoonSign{}, false
}
