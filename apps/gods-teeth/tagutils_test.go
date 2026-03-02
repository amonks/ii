package main

import "testing"

func TestNormalizeTokenParts(t *testing.T) {
	cases := []struct {
		input      string
		wantCore   string
		wantSuffix string
	}{
		{"VELASQUEZ_", "VELASQUEZ", "_"},
		{"ZION,", "ZION", ","},
		{"CDC", "CDC", ""},
		{"  FBI!)  ", "FBI", "!)"},
		{"", "", ""},
	}

	for _, tc := range cases {
		core, suffix := normalizeTokenParts(tc.input)
		if core != tc.wantCore || suffix != tc.wantSuffix {
			t.Fatalf("normalizeTokenParts(%q) = (%q, %q), want (%q, %q)", tc.input, core, suffix, tc.wantCore, tc.wantSuffix)
		}
	}
}

func TestNormalizeToken(t *testing.T) {
	if got := normalizeToken("ZION,"); got != "ZION" {
		t.Fatalf("normalizeToken(ZION,) = %q, want ZION", got)
	}
}

func TestCanonicalTagName(t *testing.T) {
	if got := canonicalTagName("STATE_RECORDS_FACILITY"); got != "STATE RECORDS FACILITY" {
		t.Fatalf("canonicalTagName returned %q", got)
	}
}
