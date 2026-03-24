package job

import (
	"testing"
	"time"
)

func TestAgeDataActiveJobUsesCreatedAt(t *testing.T) {
	createdAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	now := createdAt.Add(2 * time.Minute)

	item := Job{
		Status:    StatusActive,
		CreatedAt: createdAt,
	}

	age, ok := AgeData(item, now)
	if !ok {
		t.Fatalf("expected age data")
	}
	if age != 2*time.Minute {
		t.Fatalf("expected age 2m, got %s", age)
	}
}

func TestAgeDataCompletedJobUsesCreatedAt(t *testing.T) {
	createdAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	now := createdAt.Add(7 * time.Minute)

	item := Job{
		Status:      StatusCompleted,
		CreatedAt:   createdAt,
		CompletedAt: createdAt.Add(5 * time.Minute),
		UpdatedAt:   createdAt.Add(6 * time.Minute),
	}

	age, ok := AgeData(item, now)
	if !ok {
		t.Fatalf("expected age data")
	}
	if age != 7*time.Minute {
		t.Fatalf("expected age 7m, got %s", age)
	}
}

func TestDurationDataActiveJobUsesNow(t *testing.T) {
	createdAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	now := createdAt.Add(4 * time.Minute)

	item := Job{
		Status:    StatusActive,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}

	duration, ok := DurationData(item, now)
	if !ok {
		t.Fatalf("expected duration data")
	}
	if duration != 4*time.Minute {
		t.Fatalf("expected duration 4m, got %s", duration)
	}
}

func TestDurationDataCompletedJobUsesUpdatedAt(t *testing.T) {
	createdAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(3 * time.Minute)

	item := Job{
		Status:    StatusCompleted,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	duration, ok := DurationData(item, updatedAt.Add(time.Minute))
	if !ok {
		t.Fatalf("expected duration data")
	}
	if duration != 3*time.Minute {
		t.Fatalf("expected duration 3m, got %s", duration)
	}
}
