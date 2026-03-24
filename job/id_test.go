package job

import (
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	timestamp := time.Date(2024, 3, 2, 9, 12, 0, 0, time.UTC)

	id := GenerateID("todo-123", timestamp)

	if len(id) != 8 {
		t.Errorf("expected ID length 8, got %d: %q", len(id), id)
	}

	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= '2' && c <= '7')) {
			t.Errorf("ID contains invalid character %q: %q", c, id)
		}
	}
}

func TestGenerateID_Deterministic(t *testing.T) {
	timestamp := time.Date(2024, 3, 2, 9, 12, 0, 0, time.UTC)
	todoID := "todo-123"

	id1 := GenerateID(todoID, timestamp)
	id2 := GenerateID(todoID, timestamp)

	if id1 != id2 {
		t.Errorf("same inputs should produce same ID: got %q and %q", id1, id2)
	}
}

func TestGenerateID_DifferentInputs(t *testing.T) {
	timestamp := time.Date(2024, 3, 2, 9, 12, 0, 0, time.UTC)

	id1 := GenerateID("todo-123", timestamp)
	id2 := GenerateID("todo-999", timestamp)

	if id1 == id2 {
		t.Error("different todo IDs should produce different IDs")
	}

	id3 := GenerateID("todo-123", timestamp.Add(time.Nanosecond))
	if id1 == id3 {
		t.Error("different timestamps should produce different IDs")
	}
}
