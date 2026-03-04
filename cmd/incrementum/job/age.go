package job

import (
	"time"

	internalage "github.com/amonks/incrementum/internal/age"
)

// AgeData computes the display age and whether timing data exists.
func AgeData(item Job, now time.Time) (time.Duration, bool) {
	return internalage.AgeData(item.CreatedAt, now)
}

// Age computes the display age for a job.
func Age(item Job, now time.Time) time.Duration {
	return internalage.Age(item.CreatedAt, now)
}

// DurationData computes the display duration and whether timing data exists.
func DurationData(item Job, now time.Time) (time.Duration, bool) {
	return internalage.DurationData(item.CreatedAt, item.UpdatedAt, 0, item.Status == StatusActive, now)
}

// Duration computes the display duration for a job.
func Duration(item Job, now time.Time) time.Duration {
	duration, _ := DurationData(item, now)
	return duration
}
