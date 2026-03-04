package ids

import "testing"

func TestUniquePrefixLengths(t *testing.T) {
	ids := []string{"2u3iutfd", "2a9k1111", "abc12345"}
	lengths := UniquePrefixLengths(ids)

	if got := lengths["2u3iutfd"]; got != 2 {
		t.Fatalf("expected 2u3iutfd prefix length 2, got %d", got)
	}
	if got := lengths["2a9k1111"]; got != 2 {
		t.Fatalf("expected 2a9k1111 prefix length 2, got %d", got)
	}
	if got := lengths["abc12345"]; got != 1 {
		t.Fatalf("expected abc12345 prefix length 1, got %d", got)
	}
}

func TestNormalizeUniqueIDs(t *testing.T) {
	ids := []string{"Abc", "", "ABC", "Def", "def", "ghi"}
	got := NormalizeUniqueIDs(ids)

	want := []string{"abc", "def", "ghi"}
	if len(got) != len(want) {
		t.Fatalf("expected %d IDs, got %d", len(want), len(got))
	}
	for i, expected := range want {
		if got[i] != expected {
			t.Fatalf("expected ID %q at %d, got %q", expected, i, got[i])
		}
	}
}

func TestUniquePrefixLengthsIsCaseInsensitive(t *testing.T) {
	ids := []string{"Abc", "aBD"}
	lengths := UniquePrefixLengths(ids)

	if got := lengths["abc"]; got != 3 {
		t.Fatalf("expected abc prefix length 3, got %d", got)
	}
	if got := lengths["abd"]; got != 3 {
		t.Fatalf("expected abd prefix length 3, got %d", got)
	}
}

func TestUniquePrefixLengthsSkipsDuplicatesAndEmpty(t *testing.T) {
	ids := []string{"abc", "", "ABC"}
	lengths := UniquePrefixLengths(ids)

	if len(lengths) != 1 {
		t.Fatalf("expected 1 unique ID, got %d", len(lengths))
	}
	if got := lengths["abc"]; got != 1 {
		t.Fatalf("expected abc prefix length 1, got %d", got)
	}
}

func TestUniquePrefixLengthsNormalized(t *testing.T) {
	ids := []string{"abc123", "abd234"}
	lengths := UniquePrefixLengthsNormalized(ids)

	if got := lengths["abc123"]; got != 3 {
		t.Fatalf("expected abc123 prefix length 3, got %d", got)
	}
	if got := lengths["abd234"]; got != 3 {
		t.Fatalf("expected abd234 prefix length 3, got %d", got)
	}
}

func TestMatchPrefix(t *testing.T) {
	ids := []string{"Abc123", "def456"}
	match, found, ambiguous := MatchPrefix(ids, "ab")

	if !found {
		t.Fatalf("expected match to be found")
	}
	if ambiguous {
		t.Fatalf("expected match to be unambiguous")
	}
	if match != "Abc123" {
		t.Fatalf("expected match Abc123, got %q", match)
	}
}

func TestMatchPrefixAmbiguous(t *testing.T) {
	ids := []string{"abc123", "abd234"}
	match, found, ambiguous := MatchPrefix(ids, "a")

	if !found {
		t.Fatalf("expected match to be found")
	}
	if !ambiguous {
		t.Fatalf("expected match to be ambiguous")
	}
	if match != "" {
		t.Fatalf("expected empty match, got %q", match)
	}
}

func TestMatchPrefixNotFound(t *testing.T) {
	ids := []string{"abc123"}
	match, found, ambiguous := MatchPrefix(ids, "zzz")

	if found {
		t.Fatalf("expected match to be missing")
	}
	if ambiguous {
		t.Fatalf("expected match to be unambiguous")
	}
	if match != "" {
		t.Fatalf("expected empty match, got %q", match)
	}
}

func TestMatchPrefixNormalized(t *testing.T) {
	ids := []string{"abc123", "def456"}
	match, found, ambiguous := MatchPrefixNormalized(ids, "ABC")

	if !found {
		t.Fatalf("expected match to be found")
	}
	if ambiguous {
		t.Fatalf("expected match to be unambiguous")
	}
	if match != "abc123" {
		t.Fatalf("expected match abc123, got %q", match)
	}
}
