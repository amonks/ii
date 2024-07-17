package monitor

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
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
	for snitch, mon := range rep {
		if err := mon.Check(); err == nil {
			if err := rep.snitch(snitch); err != nil {
				return fmt.Errorf("error snitching on %s to '%s': %w", mon.Name(), snitch, err)
			}
		} else {
			log.Printf("%s", err)
		}
	}
	log.Printf("report complete in %s", time.Now().Sub(start).Truncate(time.Millisecond))
	return nil
}

func (rep Reporter) snitch(snitch string) error {
	if _, err := http.Get("https://nosnch.in/" + snitch); err != nil {
		return err
	}
	return nil
}
