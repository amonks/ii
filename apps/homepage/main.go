package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/letterboxd"
	"monks.co/pkg/ports"
	"monks.co/pkg/posts"
	"monks.co/pkg/rotate"
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
	mux.HandleFunc("GET /error/{$}", func(w http.ResponseWriter, req *http.Request) {
		code := 500
		if qCode := req.URL.Query().Get("code"); qCode != "" {
			if qCode, err := strconv.ParseInt(qCode, 10, 64); err == nil {
				code = int(qCode)
			}
		}
		serve.Errorf(w, req, code, "%d, as requested", code)
		return
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, req *http.Request) {
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
			Posts:     posts,
			Watches:   diary,
			GoSynonym: goSynonyms.Next(),
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

var goSynonyms = rotate.New[string](
	"Go",
	"囲碁",
	"いご",
	"바둑",
	"baduk",
	"圍棋",
	"wéiqí",
)
