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
		return err
	}
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
			log.Printf("success: '%s'", snitch)
			if err := rep.snitch(snitch); err != nil {
				return fmt.Errorf("error snitching to '%s': %w", snitch, err)
			}
		} else {
			log.Printf("fail: '%s': %s", snitch, err)
		}
	}
	return nil
}

func (rep Reporter) snitch(snitch string) error {
	if _, err := http.Get("https://nosnch.in/" + snitch); err != nil {
		return err
	}
	return nil
}
