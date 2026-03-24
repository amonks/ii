package ids

import (
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	id := Generate("todo-123", 8)

	if len(id) != 8 {
		t.Fatalf("expected ID length 8, got %d: %q", len(id), id)
	}

	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= '2' && c <= '7')) {
			t.Errorf("ID contains invalid character %q: %q", c, id)
		}
	}
}

func TestGenerate_Deterministic(t *testing.T) {
	id1 := Generate("todo-123", 10)
	id2 := Generate("todo-123", 10)

	if id1 != id2 {
		t.Errorf("same inputs should produce same ID: got %q and %q", id1, id2)
	}
}

func TestGenerate_DifferentInputs(t *testing.T) {
	id1 := Generate("todo-123", 10)
	id2 := Generate("todo-999", 10)

	if id1 == id2 {
		t.Error("different inputs should produce different IDs")
	}
}

func TestGenerateWithTimestamp(t *testing.T) {
	timestamp := time.Date(2024, 3, 2, 9, 12, 0, 0, time.UTC)

	id1 := GenerateWithTimestamp("todo-123", timestamp, 8)
	id2 := GenerateWithTimestamp("todo-123", timestamp, 8)
	if id1 != id2 {
		t.Errorf("same inputs should produce same ID: got %q and %q", id1, id2)
	}

	id3 := GenerateWithTimestamp("todo-123", timestamp.Add(time.Nanosecond), 8)
	if id1 == id3 {
		t.Error("different timestamps should produce different IDs")
	}
}
