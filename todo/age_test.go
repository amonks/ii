package todo

import (
	"testing"
	"time"
)

func TestAgeDataUsesCreatedAt(t *testing.T) {
	createdAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	now := createdAt.Add(12 * time.Minute)

	item := Todo{CreatedAt: createdAt}

	age, ok := AgeData(item, now)
	if !ok {
		t.Fatalf("expected age data")
	}
	if age != 12*time.Minute {
		t.Fatalf("expected age 12m, got %s", age)
	}
}

func TestAgeDataRequiresCreatedAt(t *testing.T) {
	item := Todo{}

	if _, ok := AgeData(item, time.Now()); ok {
		t.Fatalf("expected no age data")
	}
}

func TestUpdatedDataUsesUpdatedAt(t *testing.T) {
	updatedAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	now := updatedAt.Add(8 * time.Minute)

	item := Todo{UpdatedAt: updatedAt}

	age, ok := UpdatedData(item, now)
	if !ok {
		t.Fatalf("expected updated age data")
	}
	if age != 8*time.Minute {
		t.Fatalf("expected updated age 8m, got %s", age)
	}
}

func TestUpdatedDataRequiresUpdatedAt(t *testing.T) {
	item := Todo{}

	if _, ok := UpdatedData(item, time.Now()); ok {
		t.Fatalf("expected no updated age data")
	}
}

func TestDurationDataInProgressUsesNow(t *testing.T) {
	startedAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	now := startedAt.Add(5 * time.Minute)

	item := Todo{
		Status:    StatusInProgress,
		StartedAt: &startedAt,
	}

	duration, ok := DurationData(item, now)
	if !ok {
		t.Fatalf("expected duration data")
	}
	if duration != 5*time.Minute {
		t.Fatalf("expected duration 5m, got %s", duration)
	}
}

func TestDurationDataInProgressRequiresStartedAt(t *testing.T) {
	item := Todo{Status: StatusInProgress}

	if _, ok := DurationData(item, time.Now()); ok {
		t.Fatalf("expected no duration data")
	}
}

func TestDurationDataDoneUsesCompletedAt(t *testing.T) {
	startedAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(42 * time.Minute)

	item := Todo{
		Status:      StatusDone,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
	}

	duration, ok := DurationData(item, completedAt.Add(time.Minute))
	if !ok {
		t.Fatalf("expected duration data")
	}
	if duration != 42*time.Minute {
		t.Fatalf("expected duration 42m, got %s", duration)
	}
}

func TestDurationDataQueuedForMergeUsesCompletedAt(t *testing.T) {
	startedAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(30 * time.Minute)

	item := Todo{
		Status:      StatusQueuedForMerge,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
	}

	duration, ok := DurationData(item, completedAt.Add(2*time.Minute))
	if !ok {
		t.Fatalf("expected duration data")
	}
	if duration != 30*time.Minute {
		t.Fatalf("expected duration 30m, got %s", duration)
	}
}

func TestDurationDataMergingUsesCompletedAt(t *testing.T) {
	startedAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(15 * time.Minute)

	item := Todo{
		Status:      StatusMerging,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		JobID:       "job-merge",
	}

	duration, ok := DurationData(item, completedAt.Add(10*time.Minute))
	if !ok {
		t.Fatalf("expected duration data")
	}
	if duration != 15*time.Minute {
		t.Fatalf("expected duration 15m, got %s", duration)
	}
}

func TestDurationDataMergeFailedUsesCompletedAt(t *testing.T) {
	startedAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(8 * time.Minute)

	item := Todo{
		Status:      StatusMergeFailed,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		JobID:       "job-merge",
	}

	duration, ok := DurationData(item, completedAt.Add(10*time.Minute))
	if !ok {
		t.Fatalf("expected duration data")
	}
	if duration != 8*time.Minute {
		t.Fatalf("expected duration 8m, got %s", duration)
	}
}

func TestDurationDataDoneRequiresTimes(t *testing.T) {
	item := Todo{Status: StatusDone}

	if _, ok := DurationData(item, time.Now()); ok {
		t.Fatalf("expected no duration data")
	}
}
