package periodically

import (
	"context"
	"log"
	"time"
)

func Do(ctx context.Context, dur time.Duration, f func() error) error {
	if err := f(); err != nil {
		return err
	}
	t := time.NewTicker(dur)
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-t.C:
		}

		log.Println("reload")
		if err := f(); err != nil {
			return err
		}
	}
}
