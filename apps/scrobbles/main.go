package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
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
		panic(err)
	}
}

func run() error {
	port := ports.Apps["scrobbles"]
	lfm := lastfm.New(os.Getenv("LASTFM_API_KEY"))

	db, err := NewDB()
	if err != nil {
		return err
	}
	defer db.Close()

	fetch := func() error {
		var count int
		var fetchErr error
		if err := lfm.FetchRecentScrobbles("andrewmonks", func(scrobble *lastfm.Scrobble) bool {
			if err := db.AddScrobble(scrobble); err != nil && errors.Is(err, ErrDuplicate) {
				return false
			} else if err != nil && errors.Is(err, ErrStillListening) {
				return true
			} else if err != nil {
				fetchErr = err
				return false
			} else {
				count += 1
				return true
			}
		}); err != nil {
			return err
		} else if fetchErr != nil {
			return fetchErr
		}
		log.Printf("fetched %d scrobbles", count)
		if err := snitch.OK("537206854d"); err != nil {
			return err
		}
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("ok"))
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
