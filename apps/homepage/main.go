package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/letterboxd"
	"monks.co/pkg/posts"
	"monks.co/pkg/rotate"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
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

	if err := tailnet.ListenAndServe(ctx, gzip.Middleware(mux)); err != nil {
		return err
	}

	return nil
}

var goSynonyms = rotate.New(
	"Go",
	"囲碁",
	"いご",
	"바둑",
	"baduk",
	"圍棋",
	"wéiqí",
)
