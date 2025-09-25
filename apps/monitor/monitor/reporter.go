package monitor

import (
	"context"
	"fmt"
	"log"
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
	for id, mon := range rep {
		if err := mon.Check(); err == nil {
			if err := snitch.OK(id); err != nil {
				return fmt.Errorf("error snitching on %s to '%s': %w", mon.Name(), id, err)
			}
		} else {
			log.Printf("%s", err)
		}
	}
	log.Printf("report complete in %s", time.Since(start).Truncate(time.Millisecond))
	return nil
}
