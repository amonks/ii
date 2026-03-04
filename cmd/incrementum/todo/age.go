package todo

import (
	"time"

	internalage "monks.co/incrementum/internal/age"
)

// AgeData computes the display age and whether timing data exists.
func AgeData(item Todo, now time.Time) (time.Duration, bool) {
	return internalage.AgeData(item.CreatedAt, now)
}

// UpdatedData computes the display updated age and whether timing data exists.
func UpdatedData(item Todo, now time.Time) (time.Duration, bool) {
	return internalage.AgeData(item.UpdatedAt, now)
}

// DurationData computes the display duration and whether timing data exists.
func DurationData(item Todo, now time.Time) (time.Duration, bool) {
	startedAt := time.Time{}
	if item.StartedAt != nil {
		startedAt = *item.StartedAt
	}

	completedAt := time.Time{}
	if item.CompletedAt != nil {
		completedAt = *item.CompletedAt
	}

	if item.Status == StatusInProgress {
		return internalage.DurationData(startedAt, time.Time{}, 0, true, now)
	}
	if item.Status == StatusDone || item.Status == StatusQueuedForMerge || item.Status == StatusMerging || item.Status == StatusMergeFailed {
		return internalage.DurationData(startedAt, completedAt, 0, false, now)
	}
	return 0, false
}
