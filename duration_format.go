package main

import (
	"time"

	"monks.co/ii/internal/ui"
)

func formatOptionalDuration(duration time.Duration, ok bool) string {
	if !ok {
		return "-"
	}
	return ui.FormatDurationShort(duration)
}
