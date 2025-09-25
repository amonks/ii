package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
	"monks.co/credentials"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/lastfm"
	"monks.co/pkg/periodically"
	"monks.co/pkg/ports"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/snitch"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	port := ports.Apps["scrobbles"]
	lfm := lastfm.New(credentials.LastFmAPIKey)

	db, err := NewDB()
	if err != nil {
		return err
	}
	defer db.Close()

	fetch := func() error {
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
		log.Printf("fetched %d scrobbles", count)
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
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	ctx, cancel := sigctx.NewWithCancel()
	wg := new(errgroup.Group)

	wg.Go(func() error {
		if err := periodically.Do(ctx, time.Hour, fetch); err != nil {
			cancel(err)
			return err
		}
		return nil
	})

	wg.Go(func() error {
		if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
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
