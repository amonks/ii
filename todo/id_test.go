package todo

import (
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	// Test basic ID generation
	id := GenerateID("Fix login bug", timestamp)

	// ID should be 8 characters
	if len(id) != 8 {
		t.Errorf("expected ID length 8, got %d: %q", len(id), id)
	}

	// ID should be lowercase alphanumeric
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= '2' && c <= '7')) {
			t.Errorf("ID contains invalid character %q: %q", c, id)
		}
	}
}

func TestGenerateID_Deterministic(t *testing.T) {
	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	title := "Fix login bug"

	id1 := GenerateID(title, timestamp)
	id2 := GenerateID(title, timestamp)

	if id1 != id2 {
		t.Errorf("same inputs should produce same ID: got %q and %q", id1, id2)
	}
}

func TestGenerateID_DifferentInputs(t *testing.T) {
	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	id1 := GenerateID("Fix login bug", timestamp)
	id2 := GenerateID("Add feature", timestamp)

	if id1 == id2 {
		t.Error("different titles should produce different IDs")
	}

	// Different timestamps should also produce different IDs
	id3 := GenerateID("Fix login bug", timestamp.Add(time.Nanosecond))
	if id1 == id3 {
		t.Error("different timestamps should produce different IDs")
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	// Generate many IDs and check for collisions
	seen := make(map[string]struct{})
	base := time.Now()

	for i := range 1000 {
		id := GenerateID("Test todo", base.Add(time.Duration(i)*time.Nanosecond))
		if _, ok := seen[id]; ok {
			t.Errorf("collision detected for ID %q at iteration %d", id, i)
		}
		seen[id] = struct{}{}
	}
}
