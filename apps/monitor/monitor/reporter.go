package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"monks.co/pkg/snitch"
)

type Reporter map[string]Monitor

func (rep Reporter) Run(ctx context.Context, dur time.Duration) error {
	if err := rep.Report(); err != nil {
		return fmt.Errorf("monitor reporting error: %w", err)
	}
	tick := time.NewTicker(dur)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			if err := rep.Report(); err != nil {
				return fmt.Errorf("monitor reporting error: %w", err)
			}
		}
	}
}

func (rep Reporter) Report() error {
	start := time.Now()
	var failCount int
	for id, mon := range rep {
		if err := mon.Check(); err == nil {
			if err := snitch.OK(id); err != nil {
				return fmt.Errorf("error snitching on %s to '%s': %w", mon.Name(), id, err)
			}
		} else {
			failCount++
		}
	}
	if failCount > 0 {
		slog.Error("task", "task.name", "report", "task.duration_ms", time.Since(start).Milliseconds(), "task.error", fmt.Sprintf("%d checks failed", failCount))
	} else {
		slog.Info("task", "task.name", "report", "task.duration_ms", time.Since(start).Milliseconds())
	}
	return nil
}
