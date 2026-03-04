package main

import (
	"time"

	"github.com/amonks/incrementum/internal/ui"
)

func formatOptionalDuration(duration time.Duration, ok bool) string {
	if !ok {
		return "-"
	}
	return ui.FormatDurationShort(duration)
}
