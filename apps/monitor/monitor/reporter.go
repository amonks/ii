package monitor

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Reporter map[string]Monitor

func (rep Reporter) Run(ctx context.Context, dur time.Duration) error {
	tick := time.NewTicker(dur)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			if err := rep.Report(); err != nil {
				return err
			}
		}
	}
}

func (rep Reporter) Report() error {
	for snitch, mon := range rep {
		if err := mon.Check(); err == nil {
			if err := rep.snitch(snitch); err != nil {
				return fmt.Errorf("error snitching to '%s': %w", snitch, err)
			}
		}
	}
	return nil
}

func (rep Reporter) snitch(snitch string) error {
	if _, err := http.Get("https://nosnitch.in/" + snitch); err != nil {
		return err
	}
	return nil
}
