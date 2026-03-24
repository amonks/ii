package todo

import (
	"errors"
	"testing"
)

func TestIDIndexPrefixLengthsUseAllIDs(t *testing.T) {
	todos := []Todo{
		{ID: "2u3iutfd"},
		{ID: "2a9k1111"},
		{ID: "abc12345"},
	}

	index := NewIDIndex(todos)
	lengths := index.PrefixLengths()

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

func TestIDIndexResolveHandlesAmbiguousPrefixes(t *testing.T) {
	todos := []Todo{
		{ID: "2u3iutfd"},
		{ID: "2a9k1111"},
	}

	index := NewIDIndex(todos)
	_, err := index.Resolve("2")
	if err == nil {
		t.Fatalf("expected ambiguous prefix error")
	}
	if !errors.Is(err, ErrAmbiguousTodoIDPrefix) {
		t.Fatalf("expected ErrAmbiguousTodoIDPrefix, got %v", err)
	}
}

func TestIDIndexResolveMatchesCaseInsensitive(t *testing.T) {
	todos := []Todo{{ID: "2u3iutfd"}}

	index := NewIDIndex(todos)
	resolved, err := index.Resolve("2U3")
	if err != nil {
		t.Fatalf("expected resolve to succeed, got %v", err)
	}
	if resolved != "2u3iutfd" {
		t.Fatalf("expected resolved ID 2u3iutfd, got %s", resolved)
	}
}
