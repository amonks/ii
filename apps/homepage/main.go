package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/a-h/templ"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/letterboxd"
	"monks.co/pkg/ports"
	"monks.co/pkg/posts"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	ctx := sigctx.New()

	posts, err := posts.Load(ctx)
	if err != nil {
		return err
	}

	mux := serve.NewMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, req *http.Request) {
		diary, err := letterboxd.FetchDiary()
		if err != nil {
			log.Printf("letterboxd diary error: %s\n", err)
			h := templ.Handler(Homepage(&PageData{
				Posts:   posts,
				Watches: nil,
			}))
			h.ServeHTTP(w, req)
			return
		}
		h := templ.Handler(Homepage(&PageData{
			Posts:   posts,
			Watches: diary,
		}))
		h.ServeHTTP(w, req)
	})

	port := ports.Apps["homepage"]
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := serve.ListenAndServe(ctx, addr, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}

func lastFiveWatches() ([]*letterboxd.Watch, error) {
	var watches []*letterboxd.Watch
	watches, err := letterboxd.FetchDiary()
	if err != nil {
		return nil, fmt.Errorf("error fetching last 5 watches: %w", err)
	}
	return watches, nil
}

var errUnset = fmt.Errorf("unset")
