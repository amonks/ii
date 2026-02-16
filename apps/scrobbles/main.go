package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
	"monks.co/pkg/gzip"
	"monks.co/pkg/lastfm"
	"monks.co/pkg/meta"
	"monks.co/pkg/periodically"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/snitch"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("fatal", "error", err.Error(), "app.name", meta.AppName())
		}
		reqlog.Shutdown()
		os.Exit(1)
	}
}

func run() error {
	reqlog.SetupLogging()

	lfm := lastfm.New(lastFmAPIKey)

	db, err := NewDB()
	if err != nil {
		return err
	}
	defer db.Close()

	fetch := func() error {
		start := time.Now()
		var count int
		for scrobble, err := range lfm.FetchRecentScrobbles("andrewmonks") {
			if err != nil {
				return err
			}

			if err := db.AddScrobble(scrobble); err != nil && errors.Is(err, ErrDuplicate) {
				break
			} else if err != nil && errors.Is(err, ErrStillListening) {
				continue
			} else if err != nil {
				return err
			} else {
				count += 1
				continue
			}
		}
		slog.Info("task", "task.name", "fetch", "task.duration_ms", time.Since(start).Milliseconds(), "scrobbles.count", count)
		if err := snitch.OK("537206854d"); err != nil {
			return err
		}
		return nil
	}

	mux := serve.NewMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, req *http.Request) {
		scrobbles, err := db.GetScrobbles(1000, 0)
		if err != nil {
			serve.InternalServerError(w, req, err)
		}
		Index(scrobbles).Render(context.Background(), w)
	})
	ctx, cancel := sigctx.NewWithCancel()
	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}
	wg := new(errgroup.Group)

	wg.Go(func() error {
		if err := periodically.Do(ctx, time.Hour, fetch); err != nil {
			cancel(err)
			return err
		}
		return nil
	})

	wg.Go(func() error {
		if err := tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(mux))); err != nil {
			cancel(err)
			return err
		}
		return nil
	})

	if err := wg.Wait(); err != nil {
		return err
	}

	return nil
}
