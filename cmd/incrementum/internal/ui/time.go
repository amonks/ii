package ui

import (
	"fmt"
	"time"

	internalage "github.com/amonks/incrementum/internal/age"
)

// FormatTimeAgo returns a compact age string like "2m ago".
func FormatTimeAgo(then time.Time, now time.Time) string {
	age := formatTimeAge(then, now)
	if age == "-" {
		return age
	}
	return age + " ago"
}

// FormatTimeAgeShort returns a compact age string like "2m".
func FormatTimeAgeShort(then time.Time, now time.Time) string {
	return formatTimeAge(then, now)
}

// FormatDurationShort formats a duration using short units (s/m/h/d).
func FormatDurationShort(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}

	duration = duration.Truncate(time.Second)
	seconds := int64(duration.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}

	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}

	days := hours / 24
	return fmt.Sprintf("%dd", days)
}

func formatTimeAge(then time.Time, now time.Time) string {
	duration, ok := internalage.AgeData(then, now)
	if !ok {
		return "-"
	}
	return FormatDurationShort(duration)
}
